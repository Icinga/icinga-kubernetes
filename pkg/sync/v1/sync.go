package v1

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/icinga/icinga-kubernetes/pkg/com"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/icinga/icinga-kubernetes/pkg/sync"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

type Sync struct {
	db       *database.Database
	informer cache.SharedIndexInformer
	log      logr.Logger
	factory  func() schemav1.Resource
}

func NewSync(
	db *database.Database,
	informer cache.SharedIndexInformer,
	log logr.Logger,
	factory func() schemav1.Resource,
) *Sync {
	return &Sync{
		db:       db,
		informer: informer,
		log:      log,
		factory:  factory,
	}
}

func (s *Sync) Run(ctx context.Context, features ...sync.Feature) error {
	controller := sync.NewController(s.informer, s.log.WithName("controller"))

	with := sync.NewFeatures(features...)

	if !with.NoWarmup() {
		if err := s.warmup(ctx, controller); err != nil {
			return err
		}
	}

	return s.sync(ctx, controller, features...)
}

func (s *Sync) warmup(ctx context.Context, c *sync.Controller) error {
	g, ctx := errgroup.WithContext(ctx)

	entities, errs := s.db.YieldAll(ctx, func() (interface{}, error) {
		return s.factory(), nil
	}, s.db.BuildSelectStmt(s.factory(), &schemav1.Meta{}))
	// Let errors from YieldAll() cancel the group.
	com.ErrgroupReceive(g, errs)

	g.Go(func() error {
		defer runtime.HandleCrash()

		for {
			select {
			case e, more := <-entities:
				if !more {
					return nil
				}

				if err := c.Announce(e); err != nil {
					fmt.Println(err)
					return err
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return g.Wait()
}

func (s *Sync) sync(ctx context.Context, c *sync.Controller, features ...sync.Feature) error {
	sink := sync.NewSink(func(i *sync.Item) interface{} {
		entity := s.factory()
		entity.Obtain(i.Item)

		return entity
	}, func(k interface{}) interface{} {
		return types.Checksum(k)
	})

	with := sync.NewFeatures(features...)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		defer runtime.HandleCrash()

		err := c.Stream(ctx, sink)
		if err != nil {
			fmt.Println(err)
		}
		return err
	})
	g.Go(func() error {
		defer runtime.HandleCrash()

		err := s.db.UpsertStreamed(
			ctx, sink.UpsertCh(),
			database.WithCascading(), database.WithOnSuccess(with.OnUpsert()))
		if err != nil {
			fmt.Println(err)
		}
		return err
	})
	g.Go(func() error {
		defer runtime.HandleCrash()

		if with.NoDelete() {
			for {
				select {
				case _, more := <-sink.DeleteCh():
					if !more {
						return nil
					}
				case <-ctx.Done():
					return ctx.Err()
				}

			}
		} else {
			err := s.db.DeleteStreamed(
				ctx, s.factory(), sink.DeleteCh(),
				database.WithBlocking(), database.WithCascading(), database.WithOnSuccess(with.OnDelete()))
			if err != nil {
				fmt.Println(err)
			}
			return err
		}
	})
	g.Go(func() error {
		defer runtime.HandleCrash()

		for {
			select {
			case err, more := <-sink.ErrorCh():
				if !more {
					return nil
				}

				s.log.Error(err, "sync error")
			case <-ctx.Done():
				return ctx.Err()
			}

		}
	})

	err := g.Wait()
	if err != nil {
		fmt.Println(err)
	}
	return err
}
