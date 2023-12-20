package v1

import (
	"context"
	"database/sql"
	"errors"
	"github.com/go-co-op/gocron"
	"github.com/icinga/icinga-kubernetes/pkg/com"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	"golang.org/x/sync/errgroup"
	"io"
	kcorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"sync"
	"time"
)

var (
	scheduler = gocron.NewScheduler(time.UTC)

	containerLogs   = make(map[string]ContainerLog)
	containerLogsMu sync.Mutex

	deletedPodIds = make(map[string]bool)
)

const (
	MaxConcurrentJobs int = 60
	ScheduleInterval      = 5 * time.Minute
)

type ContainerMeta struct {
	Id    types.Binary `db:"id"`
	PodId types.Binary `db:"pod_id"`
}

type Container struct {
	ContainerMeta
	Name           string
	Image          string
	CpuLimits      int64
	CpuRequests    int64
	MemoryLimits   int64
	MemoryRequests int64
	State          sql.NullString
	StateDetails   string
	Ready          types.Bool
	Started        types.Bool
	RestartCount   int32
	Devices        []ContainerDevice `db:"-"`
	Mounts         []ContainerMount  `db:"-"`
}

func (c *Container) Relations() []database.Relation {
	fk := database.WithForeignKey("container_id")

	return []database.Relation{
		database.HasMany(c.Devices, fk),
		database.HasMany(c.Mounts, fk),

		// Allow to automatically remove the logs when a container is deleted. Otherwise, we will have some dangling
		// container logs in the database if the logs aren't deleted before removing the container, since any error
		// can interrupt the deletion process of the logs when using the `on success` mechanism.
		database.HasOne(ContainerLog{}, fk),
	}
}

type ContainerDevice struct {
	ContainerId types.Binary
	PodId       types.Binary
	Name        string
	Path        string
}

type ContainerMount struct {
	ContainerId types.Binary
	PodId       types.Binary
	VolumeName  string
	Path        string
	SubPath     sql.NullString
	ReadOnly    types.Bool
}

type ContainerLogMeta struct {
	Logs       string          `db:"logs"`
	LastUpdate types.UnixMilli `db:"last_update"`
}

type ContainerLog struct {
	PodId       types.Binary `db:"pod_id"`
	ContainerId types.Binary `db:"container_id"`
	ContainerLogMeta

	Namespace     string `db:"-"`
	PodName       string `db:"-"`
	ContainerName string `db:"-"`
}

// Upsert implements the database.Upserter interface.
func (cl *ContainerLog) Upsert() interface{} {
	return cl.ContainerLogMeta
}

// syncContainerLogs fetches the logs from the kubernetes API for the given container and syncs to the database.
func (cl *ContainerLog) syncContainerLogs(ctx context.Context, clientset *kubernetes.Clientset, db *database.Database) error {
	logOptions := &kcorev1.PodLogOptions{Container: cl.ContainerName}
	if !cl.LastUpdate.Time().IsZero() {
		logOptions.SinceSeconds = pointer.Int64(int64(time.Since(cl.LastUpdate.Time()).Seconds()))
	}

	req := clientset.CoreV1().Pods(cl.Namespace).GetLogs(cl.PodName, logOptions)
	body, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()

	logs, err := io.ReadAll(body)
	if err != nil || len(logs) == 0 {
		return err
	}

	cl.LastUpdate = types.UnixMilli(time.Now())
	cl.Logs += string(logs)
	entities := make(chan interface{}, 1)
	entities <- cl
	close(entities)

	return db.UpsertStreamed(ctx, entities)
}

// SyncContainers consumes from the `upsertPods` and `deletePods` chans concurrently and schedules a job for
// each of the containers (drawn from `upsertPods`) that periodically syncs the container logs with the database.
// When pods are deleted, their IDs are streamed through the `deletePods` chan, and this fetches all the container
// IDs matching the respective pod ID from the database and initiates a container deletion stream that cleans up all
// container-related resources.
func SyncContainers(ctx context.Context, db *database.Database, g *errgroup.Group, upsertPods <-chan interface{}, deletePods <-chan interface{}) {
	// Fetch all container logs from the database
	err := make(chan error, 1)
	err <- warmup(ctx, db)
	close(err)
	com.ErrgroupReceive(g, err)

	// Use buffered channel here not to block the goroutines, as they can stream container ids
	// from multiple pods concurrently.
	containerIds := make(chan interface{}, db.Options.MaxPlaceholdersPerStatement)
	g.Go(func() error {
		defer runtime.HandleCrash()

		return db.DeleteStreamed(ctx, &Container{}, containerIds, database.WithCascading())
	})

	g.Go(func() error {
		defer runtime.HandleCrash()
		defer close(containerIds)

		scheduler.SetMaxConcurrentJobs(MaxConcurrentJobs, gocron.WaitMode)
		scheduler.TagsUnique()

		scheduler.StartAsync()
		defer scheduler.Stop()

		query := db.BuildSelectStmt(&Container{}, ContainerMeta{}) + ` WHERE pod_id=:pod_id`

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case podId, ok := <-deletePods:
				if !ok {
					return nil
				}

				meta := &ContainerMeta{PodId: podId.([]byte)}
				if _, ok := deletedPodIds[meta.PodId.String()]; ok {
					// Due to the recursive relation resolution in the `DB#DeleteStreamed()` method, we may get the
					// same pod ID multiple times since they all share the same `on success` handler.
					break
				}
				deletedPodIds[meta.PodId.String()] = true

				entities, errs := db.YieldAll(ctx, func() (interface{}, error) {
					return &Container{}, nil
				}, query, meta)
				com.ErrgroupReceive(g, errs)

				g.Go(func() error {
					defer runtime.HandleCrash()

					for {
						select {
						case <-ctx.Done():
							return ctx.Err()
						case e, ok := <-entities:
							if !ok {
								return nil
							}

							container := e.(*Container)
							select {
							case containerIds <- container.Id:
							case <-ctx.Done():
								return ctx.Err()
							}

							err := scheduler.RemoveByTag(container.Id.String())
							if err != nil && !errors.Is(err, gocron.ErrJobNotFoundWithTag) {
								return err
							}

							containerLogsMu.Lock()
							delete(containerLogs, container.Id.String())
							containerLogsMu.Unlock()
						}
					}
				})
			case e, ok := <-upsertPods:
				if !ok {
					return nil
				}

				pod := e.(*Pod)

				delete(deletedPodIds, pod.Id.String())

				for _, container := range pod.Containers {
					_, err := scheduler.FindJobsByTag(container.Id.String())
					if err != nil && !errors.Is(err, gocron.ErrJobNotFoundWithTag) {
						return err
					}

					if container.Started.Bool && err != nil {
						containerLog := &ContainerLog{
							ContainerId:   container.Id,
							PodId:         container.PodId,
							ContainerName: container.Name,
							Namespace:     pod.Namespace,
							PodName:       pod.Name,
						}

						containerLogsMu.Lock()
						if cl, ok := containerLogs[container.Id.String()]; ok {
							containerLog.Logs = cl.Logs
						}
						containerLogsMu.Unlock()

						scheduler.Every(ScheduleInterval.String()).Tag(container.Id.String())
						_, err = scheduler.Do(containerLog.syncContainerLogs, ctx, pod.factory.clientset, db)
						if err != nil {
							return err
						}
					} else if err == nil {
						err := scheduler.RemoveByTag(container.Id.String())
						if err != nil {
							return err
						}

						containerLogsMu.Lock()
						delete(containerLogs, container.Id.String())
						containerLogsMu.Unlock()
					}
				}
			}
		}
	})
}

// warmup fetches all container logs from the database and caches them in the containerlogs variable.
func warmup(ctx context.Context, db *database.Database) error {
	g, ctx := errgroup.WithContext(ctx)

	entities, errs := db.YieldAll(ctx, func() (interface{}, error) {
		return &ContainerLog{}, nil
	}, db.BuildSelectStmt(ContainerLog{}, ContainerLog{}))
	com.ErrgroupReceive(g, errs)

	g.Go(func() error {
		defer runtime.HandleCrash()

		containerLogsMu.Lock()
		defer containerLogsMu.Unlock()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case e, ok := <-entities:
				if !ok {
					return nil
				}

				containerLog := e.(*ContainerLog)
				containerLogs[containerLog.ContainerId.String()] = *containerLog
			}
		}
	})

	return g.Wait()
}

// Assert that the Container type satisfies the interface compliance.
var (
	_ database.HasRelations = (*Container)(nil)
)
