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
	Run(context.Context, ...syncOption) error
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
	return &sync{
		db:       db,
		informer: informer,
		logger:   logger,
		factory:  factory,
	}
}

func (s *sync) Run(ctx context.Context, options ...syncOption) error {
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

	syncOpts := newSyncOptions(options...)

	kupsertsMux := NewChannelMux(changes.Adds(), changes.Updates())
	kupserts := kupsertsMux.Out()

	if syncOpts.forwardUpserts != nil {
		kupsertsMux.AddOut(syncOpts.forwardUpserts)
	}

	g.Go(func() error {
		return kupsertsMux.Run(ctx)
	})

	databaseUpserts := make(chan database.Entity)
	defer close(databaseUpserts)

	g.Go(func() error {
		for {
			select {
			case kupsert, more := <-kupserts:
				if !more {
					return nil
				}

				entity := s.factory()
				entity.SetID(kupsert.ID())
				entity.SetCanonicalName(kupsert.GetCanonicalName())
				entity.Obtain(kupsert.KObject())

				select {
				case databaseUpserts <- entity:
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

	g.Go(func() error {
		return s.db.UpsertStreamed(ctx, databaseUpserts)
	})

	kdeletesMux := NewChannelMux(changes.Deletes())
	kdeletes := kdeletesMux.Out()

	if syncOpts.forwardDeletes != nil {
		kdeletesMux.AddOut(syncOpts.forwardDeletes)
	}

	g.Go(func() error {
		return kdeletesMux.Run(ctx)
	})

	databaseDeletes := make(chan any)
	g.Go(func() error {
		defer close(databaseDeletes)

		for {
			select {
			case kdelete, more := <-kdeletes:
				if !more {
					return nil
				}

				select {
				case databaseDeletes <- kdelete.ID():
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
		return s.db.DeleteStreamed(ctx, s.factory(), databaseDeletes)
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
