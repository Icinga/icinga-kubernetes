package sync

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/sink"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	kcache "k8s.io/client-go/tools/cache"
)

type Sync interface {
	Run(context.Context, ...SyncOption) error
}

type sync struct {
	db       *database.DB
	factory  func() contracts.Resource
	informer kcache.SharedInformer
	logger   *logging.Logger
}

func NewSync(
	db *database.DB,
	factory func() contracts.Resource,
	informer kcache.SharedInformer,
	logger *logging.Logger,
) Sync {
	s := &sync{
		db:       db,
		informer: informer,
		logger:   logger,
		factory:  factory,
	}

	return s
}

func WithForwardUpsertToLog(channel chan<- contracts.KUpsert) SyncOption {
	return func(options *SyncOptions) {
		options.forwardUpsertToLogChannel = channel
	}
}

func WithForwardDeleteToLog(channel chan<- contracts.KDelete) SyncOption {
	return func(options *SyncOptions) {
		options.forwardDeleteToLogChannel = channel
	}
}

type SyncOption func(options *SyncOptions)

type SyncOptions struct {
	forwardUpsertToLogChannel chan<- contracts.KUpsert
	forwardDeleteToLogChannel chan<- contracts.KDelete
}

func NewOptionStorage(execOptions ...SyncOption) *SyncOptions {
	optionStorage := &SyncOptions{}

	for _, option := range execOptions {
		option(optionStorage)
	}

	return optionStorage
}

func (s *sync) Run(ctx context.Context, execOptions ...SyncOption) error {
	s.logger.Info("Starting sync")

	s.logger.Debug("Warming up")

	err := s.Warmup(ctx)
	if err != nil {
		return errors.Wrap(err, "warmup failed")
	}

	changes := sink.NewSink(s.informer.GetStore(), s.logger)
	if _, err := s.informer.AddEventHandler(changes); err != nil {
		return errors.Wrap(err, "can't add event handlers")
	}

	g, ctx := errgroup.WithContext(ctx)

	s.logger.Debug("Starting informer")
	go s.informer.Run(ctx.Done())

	if !kcache.WaitForCacheSync(ctx.Done(), s.informer.HasSynced) {
		return errors.New("timed out waiting for caches to sync")
	}

	s.logger.Debug("Finished warming up")

	s.factory().GetResourceVersion()

	syncOptions := NewOptionStorage(execOptions...)

	// init upsert channel spreader
	multiplexUpsertChannel := make(chan contracts.KUpsert)
	defer close(multiplexUpsertChannel)

	multiplexUpsert := NewChannelSpreader[contracts.KUpsert](multiplexUpsertChannel)

	upsertChannel := multiplexUpsert.NewChannel()

	if syncOptions.forwardUpsertToLogChannel != nil {
		multiplexUpsert.AddChannel(syncOptions.forwardUpsertToLogChannel)
	}

	// run upsert channel spreader
	g.Go(func() error {
		return multiplexUpsert.Run(ctx)
	})

	upsertToStream := make(chan database.Entity)
	defer close(upsertToStream)

	for _, ch := range []<-chan contracts.KUpsert{changes.Adds(), changes.Updates()} {
		ch := ch

		g.Go(func() error {
			for {
				select {
				case kupsert, more := <-ch:
					if !more {
						return nil
					}

					select {
					case multiplexUpsertChannel <- kupsert:
					case <-ctx.Done():
						return ctx.Err()
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		g.Go(func() error {
			for {
				select {
				case kupsert, more := <-upsertChannel:
					if !more {
						return nil
					}

					entity := s.factory()
					entity.SetID(kupsert.ID())
					entity.SetCanonicalName(kupsert.GetCanonicalName())
					entity.Obtain(kupsert.KObject())

					select {
					case upsertToStream <- entity:
						s.logger.Debugw(
							fmt.Sprintf("Sync: Upserted %s", kupsert.GetCanonicalName()),
							zap.String("id", kupsert.ID().String()))
					case <-ctx.Done():
						return ctx.Err()
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})
	}

	g.Go(func() error {
		return s.db.UpsertStreamed(ctx, upsertToStream)
	})

	// init delete channel spreader
	multiplexDeleteChannel := make(chan contracts.KDelete)
	defer close(multiplexDeleteChannel)

	multiplexDelete := NewChannelSpreader[contracts.KDelete](multiplexDeleteChannel)

	deleteChannel := multiplexDelete.NewChannel()

	if syncOptions.forwardDeleteToLogChannel != nil {
		multiplexDelete.AddChannel(syncOptions.forwardDeleteToLogChannel)
	}

	// run delete channel spreader
	g.Go(func() error {
		return multiplexDelete.Run(ctx)
	})

	g.Go(func() error {
		for {
			select {
			case kdelete, more := <-changes.Deletes():
				if !more {
					return nil
				}
				select {
				case multiplexDeleteChannel <- kdelete:
				case <-ctx.Done():
					return ctx.Err()
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	deleteToStream := make(chan any)
	g.Go(func() error {
		defer close(deleteToStream)

		for {
			select {
			case kdelete, more := <-deleteChannel:
				if !more {
					return nil
				}

				select {
				case deleteToStream <- kdelete.ID():
					s.logger.Debugw(
						fmt.Sprintf("Sync: Deleted %s", kdelete.GetCanonicalName()),
						zap.String("id", kdelete.ID().String()))
				case <-ctx.Done():
					return ctx.Err()
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	g.Go(func() error {
		return s.db.DeleteStreamed(ctx, s.factory(), deleteToStream)
	})

	g.Go(func() error {
		return changes.Run(ctx)
	})

	return g.Wait()
}

func (s *sync) Warmup(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	resource := s.factory()
	entities, err := s.db.YieldAll(ctx, func() database.Entity {
		return s.factory()
	}, s.db.BuildSelectStmt(resource, resource.Fingerprint()), struct{}{})
	com.ErrgroupReceive(ctx, g, err)

	g.Go(func() error {
		for {
			select {
			case e, more := <-entities:
				if !more {
					return nil
				}

				if err := s.informer.GetStore().Add(e); err != nil {
					return err
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return g.Wait()
}
