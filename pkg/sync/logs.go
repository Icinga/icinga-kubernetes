package sync

import (
	"bufio"
	"context"
	"crypto/sha1"
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
	"strings"
	msync "sync"
	"time"
)

type podListItem struct {
	pod            *kcorev1.Pod
	lastTimestamps map[[20]byte]*kmetav1.Time
}

// ContainerLogSync reacts to pod changes and syncs container logs to database.
// On pod add/updates ContainerLogSync starts syncing. On pod deletes syncing stops.
// Container logs are periodic fetched from Kubernetes API.
type ContainerLogSync interface {
	// Run starts the ContainerLogSync.
	Run(context.Context, <-chan contracts.KUpsert, <-chan contracts.KDelete) error
}

// containerLogSync syncs container logs to database.
type containerLogSync struct {
	pods      map[[20]byte]podListItem
	mutex     *msync.RWMutex
	clientset *kubernetes.Clientset
	db        *database.DB
	logger    *logging.Logger
}

// NewContainerLogSync creates new containerLogSync initialized with clientset, database and logger.
func NewContainerLogSync(clientset *kubernetes.Clientset, db *database.DB, logger *logging.Logger) ContainerLogSync {
	return &containerLogSync{
		pods:      make(map[[20]byte]podListItem),
		mutex:     &msync.RWMutex{},
		clientset: clientset,
		db:        db,
		logger:    logger,
	}
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

// splitTimestampsFromMessages takes a log and returns timestamps and messages as separate parts.
// Additionally, it updates the last checked timestamp for the container log.
func (ls *containerLogSync) splitTimestampsFromMessages(log types.Binary, curPodId [20]byte, curContainerId [20]byte) (times []string, messages []string, err error) {
	stringReader := strings.NewReader(string(log))
	reader := bufio.NewReader(stringReader)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, nil, errors.Wrap(err, "error reading log message")
		}

		messageTime, err := time.Parse("2006-01-02T15:04:05.999999999Z", strings.Split(line, " ")[0])
		if err != nil {
			logging.Fatal(errors.Wrap(err, "error parsing log timestamp"))
			continue
		}

		if ls.pods[curPodId].lastTimestamps[curContainerId] != nil && messageTime.UnixNano() <= ls.pods[curPodId].lastTimestamps[curContainerId].UnixNano() {
			continue
		}

		times = append(times, strings.Split(line, " ")[0])
		messages = append(messages, strings.Join(strings.Split(line, " ")[1:], " "))
	}

	return times, messages, nil
}

// maintainList updates pods depending on the objects coming in via upsert and delete channel.
func (ls *containerLogSync) maintainList(ctx context.Context, upsertChannel <-chan contracts.KUpsert, deleteChannel <-chan contracts.KDelete) error {
	g, ctx := errgroup.WithContext(ctx)

	deletes := make(chan any)
	g.Go(func() error {
		defer close(deletes)

		for {
			select {
			case <-ctx.Done():
				return errors.Wrap(ctx.Err(), "context canceled maintain log sync pods")

			case podFromChannel, more := <-upsertChannel:
				if !more {
					return nil
				}

				pod := podFromChannel.KObject().(*kcorev1.Pod)
				podId := sha1.Sum(types.Checksum(podFromChannel.ID().String()))

				_, ok := ls.pods[podId]

				if ok {
					continue
				}

				ls.mutex.RLock()
				ls.pods[podId] = podListItem{pod: pod}
				ls.mutex.RUnlock()

			case podIdFromChannel, more := <-deleteChannel:
				if !more {
					return nil
				}

				podId := sha1.Sum(types.Checksum(podIdFromChannel.ID().String()))

				ls.mutex.RLock()
				delete(ls.pods, podId)
				ls.mutex.RUnlock()

				select {
				case deletes <- podId:
				case <-ctx.Done():
					return ctx.Err()
				}
			}

		}
	})

	g.Go(func() error {
		return database.NewDelete(ls.db).ByColumn("container_id").Stream(ctx, &schema.ContainerLog{}, deletes)
	})

	return g.Wait()
}

func (ls *containerLogSync) Run(ctx context.Context, upsertChannel <-chan contracts.KUpsert, deleteChannel <-chan contracts.KDelete) error {
	ls.logger.Info("Starting sync")

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return ls.maintainList(ctx, upsertChannel, deleteChannel)
	})

	upsertStmt := ls.upsertStmt()
	upserts := make(chan database.Entity)
	defer close(upserts)

	g.Go(func() error {
		for {
			for _, element := range ls.pods {
				podId := sha1.Sum(types.Checksum(element.pod.Namespace + "/" + element.pod.Name))
				for _, container := range element.pod.Spec.Containers {
					containerId := sha1.Sum(types.Checksum(element.pod.Namespace + "/" + element.pod.Name + "/" + container.Name))
					podLogOpts := kcorev1.PodLogOptions{Container: container.Name, Timestamps: true}

					if ls.pods[podId].lastTimestamps != nil {
						podLogOpts.SinceTime = ls.pods[podId].lastTimestamps[containerId]
					}

					log, err := ls.clientset.CoreV1().Pods(element.pod.Namespace).GetLogs(element.pod.Name, &podLogOpts).Do(ctx).Raw()
					if err != nil {
						fmt.Println(errors.Wrap(err, "error reading container log"))
						continue
					}

					times, messages, err := ls.splitTimestampsFromMessages(log, podId, containerId)
					if err != nil {
						return err
					}

					if len(messages) == 0 {
						continue
					}

					newLog := &schema.ContainerLog{
						ContainerId: containerId[:],
						PodId:       podId[:],
						Time:        strings.Join(times, "\n"),
						Log:         strings.Join(messages, "\n"),
					}

					select {
					case upserts <- newLog:
					case <-ctx.Done():
						return ctx.Err()

					}

					lastTime, err := time.Parse("2006-01-02T15:04:05.999999999Z", times[len(times)-1])
					if err != nil {
						return errors.Wrap(err, "error parsing log time")
					}

					lastV1Time := kmetav1.Time{Time: lastTime}

					if _, ok := ls.pods[podId]; !ok {
						continue
					}

					ls.pods[podId].lastTimestamps[containerId] = &lastV1Time
				}
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second * 15):
			}
		}
	})

	g.Go(func() error {
		return database.NewUpsert(ls.db).WithStatement(upsertStmt, 5).Stream(ctx, upserts)
	})

	return g.Wait()
}
