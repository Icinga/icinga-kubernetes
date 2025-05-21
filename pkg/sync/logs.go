package sync

import (
	"bufio"
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/schema"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"io"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strconv"
	"strings"
	gosync "sync"
	"time"
)

// ContainerLogSync reacts to pod changes and synchronizes container logs
// with the database. When a pod is added/updated, ContainerLogSync starts
// synchronizing its containers. When a pod is deleted, synchronization stops.
// Container logs are periodically fetched from the Kubernetes API.
type ContainerLogSync interface {
	// Run starts the ContainerLogSync.
	Run(context.Context, <-chan contracts.KUpsert, <-chan contracts.KDelete) error
}

// NewContainerLogSync creates new ContainerLogSync initialized with clientset, database and logger.
func NewContainerLogSync(clientset *kubernetes.Clientset, db *database.DB, logger *logging.Logger, period time.Duration) ContainerLogSync {
	return &containerLogSync{
		pods:      make(map[string]podListItem),
		mutex:     &gosync.RWMutex{},
		clientset: clientset,
		db:        db,
		logger:    logger,
		period:    period,
	}
}

// containerLogSync syncs container logs to database.
type containerLogSync struct {
	pods      map[string]podListItem
	mutex     *gosync.RWMutex
	clientset *kubernetes.Clientset
	db        *database.DB
	logger    *logging.Logger
	period    time.Duration
}

type podListItem struct {
	pod            *kcorev1.Pod
	lastTimestamps map[string]*kmetav1.Time
}

// upsertStmt returns a database statement to upsert a container log.
func (ls *containerLogSync) upsertStmt() string {
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		"container_log",
		"container_id, pod_id, time, log",
		":container_id, :pod_id, :time, :log",
		"time=CONCAT(time, '\n', :time), log=CONCAT(log, '\n', :log)",
	)
}

// splitTimestampsFromMessages takes a log line and returns timestamps and messages as separate parts.
func (ls *containerLogSync) splitTimestampsFromMessages(log types.Binary, curPodId string, curContainerId string) (times []string, messages []string, newLastTimestamp time.Time, returnErr error) {
	stringReader := strings.NewReader(string(log))
	reader := bufio.NewReader(stringReader)

	var parsedTimestamp time.Time

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}

			returnErr = errors.Wrap(err, "error reading log message")
			return
		}

		timestamp, message, _ := strings.Cut(line, " ")

		parsedTimestamp, err = time.Parse("2006-01-02T15:04:05.999999999Z", timestamp)
		if err != nil {
			ls.logger.Fatal(errors.Wrap(err, "error parsing log timestamp"))
			continue
		}

		if lastTimestamp, ok := ls.pods[curPodId].lastTimestamps[curContainerId]; ok &&
			(parsedTimestamp.Before(lastTimestamp.Time) || parsedTimestamp.Equal(lastTimestamp.Time)) {
			continue
		}

		times = append(times, strconv.FormatInt(parsedTimestamp.UnixMilli(), 10))
		messages = append(messages, message)
	}

	newLastTimestamp = parsedTimestamp

	return
}

// maintainList updates pods depending on the objects coming in via upsert and delete channel.
func (ls *containerLogSync) maintainList(ctx context.Context, kupserts <-chan contracts.KUpsert, kdeletes <-chan contracts.KDelete) error {
	g, ctx := errgroup.WithContext(ctx)

	databaseDeletes := make(chan any)
	g.Go(func() error {
		defer close(databaseDeletes)

		for {
			select {
			case kupsert, more := <-kupserts:
				if !more {
					return nil
				}

				podId := kupsert.ID().String()

				if _, ok := ls.pods[podId]; ok {
					continue
				}

				ls.mutex.RLock()
				ls.pods[podId] = podListItem{
					pod:            kupsert.KObject().(*kcorev1.Pod),
					lastTimestamps: make(map[string]*kmetav1.Time),
				}
				ls.mutex.RUnlock()

			case kdelete, more := <-kdeletes:
				if !more {
					return nil
				}

				podId := kdelete.ID().String()

				ls.mutex.RLock()
				delete(ls.pods, podId)
				ls.mutex.RUnlock()

				select {
				case databaseDeletes <- podId:
				case <-ctx.Done():
					return ctx.Err()
				}
			case <-ctx.Done():
				return ctx.Err()
			}

		}
	})

	g.Go(func() error {
		return database.NewDelete(ls.db, database.ByColumn("container_id")).Stream(ctx, &schema.ContainerLog{}, databaseDeletes)
	})

	return g.Wait()
}

func (ls *containerLogSync) Run(ctx context.Context, kupserts <-chan contracts.KUpsert, kdeletes <-chan contracts.KDelete) error {
	ls.logger.Info("Starting sync")

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return ls.maintainList(ctx, kupserts, kdeletes)
	})

	databaseUpserts := make(chan database.Entity)
	defer close(databaseUpserts)

	g.Go(func() error {
		for {
			for _, element := range ls.pods {
				podId := types.Binary(types.Checksum(element.pod.Namespace + "/" + element.pod.Name))
				for _, container := range element.pod.Spec.Containers {
					containerId := types.Binary(types.Checksum(element.pod.Namespace + "/" + element.pod.Name + "/" + container.Name))
					podLogOpts := kcorev1.PodLogOptions{Container: container.Name, Timestamps: true}

					if _, ok := ls.pods[podId.String()].lastTimestamps[containerId.String()]; ok {
						podLogOpts.SinceTime = ls.pods[podId.String()].lastTimestamps[containerId.String()]
					}

					log, err := ls.clientset.CoreV1().Pods(element.pod.Namespace).GetLogs(element.pod.Name, &podLogOpts).Do(ctx).Raw()
					if err != nil {
						ls.logger.Fatal(errors.Wrap(err, "error reading container log"))
						continue
					}

					times, messages, lastTimestamp, err := ls.splitTimestampsFromMessages(log, podId.String(), containerId.String())
					if err != nil {
						return err
					}

					if len(messages) == 0 {
						continue
					}

					newLog := &schema.ContainerLog{
						ContainerId: containerId,
						PodId:       podId,
						Time:        strings.Join(times, "\n"),
						Log:         strings.Join(messages, "\n"),
					}

					select {
					case databaseUpserts <- newLog:
					case <-ctx.Done():
						return ctx.Err()

					}

					if _, ok := ls.pods[podId.String()]; !ok {
						continue
					}

					ls.pods[podId.String()].lastTimestamps[containerId.String()] = &kmetav1.Time{Time: lastTimestamp}
				}
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(ls.period):
			}
		}
	})

	g.Go(func() error {
		return database.NewUpsert(ls.db, database.WithStatement(ls.upsertStmt(), 5)).Stream(ctx, databaseUpserts)
	})

	return g.Wait()
}
