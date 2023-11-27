package sync

import (
	"bufio"
	"context"
	"crypto/sha1"
	"fmt"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/schema"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"io"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"slices"
	"strings"
	msync "sync"
	"time"
)

// LogSync syncs logs to database. Therefore, it maintains a list
// of pod elements to get logs from
type LogSync struct {
	list        []*kcorev1.Pod
	lastChecked map[[20]byte]*kmetav1.Time
	mutex       *msync.RWMutex
	clientset   *kubernetes.Clientset
	db          *database.DB
	logger      *logging.Logger
}

// NewLogSync creates new LogSync initialized with clientset, database and logger
func NewLogSync(clientset *kubernetes.Clientset, db *database.DB, logger *logging.Logger) *LogSync {
	return &LogSync{
		list:        []*kcorev1.Pod{},
		lastChecked: make(map[[20]byte]*kmetav1.Time),
		mutex:       &msync.RWMutex{},
		clientset:   clientset,
		db:          db,
		logger:      logger,
	}
}

// upsertStmt returns database upsert statement
func (ls *LogSync) upsertStmt() string {
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		"log",
		"id, reference_id, container_name, time, log",
		":id, :reference_id, :container_name, :time, :log",
		"time=CONCAT(time, '\n', :time), log=CONCAT(log, '\n', :log)",
	)
}

// splitTimestampsFromMessages takes a log as []byte and returns timestamps and messages as separate string slices.
// Additionally, it updates the last checked timestamp for the log
func (ls *LogSync) splitTimestampsFromMessages(log []byte, curContainerId [20]byte) (times []string, messages []string, err error) {

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

		if ls.lastChecked[curContainerId] != nil && messageTime.UnixNano() <= ls.lastChecked[curContainerId].UnixNano() {
			continue
		}

		times = append(times, strings.Split(line, " ")[0])
		messages = append(messages, strings.Join(strings.Split(line, " ")[1:], " "))
	}

	return times, messages, nil
}

// removeFromList removes pod from maintained list
func (ls *LogSync) removeFromList(id database.ID) {
	out := make([]*kcorev1.Pod, 0)

	for _, element := range ls.list {

		elementId := sha1.Sum([]byte(element.Namespace + "/" + element.Name))

		if fmt.Sprintf("%x", elementId) != id.String() {
			out = append(out, element)
		}
	}

	ls.list = out
}

// MaintainList adds pods from the addChannel to the list and deletes pods from the deleteChannel from the list
func (ls *LogSync) MaintainList(ctx context.Context, addChannel <-chan contracts.KUpsert, deleteChannel <-chan contracts.KDelete) error {

	ls.logger.Info("Starting maintain list")

	g, ctx := errgroup.WithContext(ctx)

	deletes := make(chan any)
	g.Go(func() error {
		defer close(deletes)

		for {
			select {
			case <-ctx.Done():
				return errors.Wrap(ctx.Err(), "context canceled maintain log sync list")

			case podFromChannel, more := <-addChannel:
				if !more {
					return nil
				}

				pod := podFromChannel.KObject().(*kcorev1.Pod)

				podIsInList := false

				for _, listPod := range ls.list {
					if listPod.UID == pod.UID {
						podIsInList = true
					}
				}

				if podIsInList {
					continue
				}

				ls.mutex.RLock()
				ls.list = append(ls.list, pod)
				ls.mutex.RUnlock()

			case podIdFromChannel, more := <-deleteChannel:
				if !more {
					return nil
				}

				idOfPod := podIdFromChannel.ID()

				ls.mutex.RLock()
				ls.removeFromList(idOfPod)
				ls.mutex.RUnlock()

				deletes <- idOfPod
			}

		}
	})

	g.Go(func() error {
		return ls.db.DeleteStreamed(ctx, &schema.Log{}, deletes, database.ByColumn("reference_id"))
	})

	return g.Wait()
}

// Run starts syncing the logs to the database. Therefore, it loops over all
// containers of each pod in the maintained list every 15 seconds.
func (ls *LogSync) Run(ctx context.Context) error {

	ls.logger.Info("Starting sync")

	g, ctx := errgroup.WithContext(ctx)

	upsertStmt := ls.upsertStmt()

	upserts := make(chan database.Entity)
	defer close(upserts)

	g.Go(func() error {
		for {
			for _, pod := range ls.list {

				curPodId := sha1.Sum([]byte(pod.Namespace + "/" + pod.Name))

				for _, container := range pod.Spec.Containers {

					curContainerId := sha1.Sum([]byte(pod.Namespace + "/" + pod.Name + "/" + container.Name))

					podLogOpts := kcorev1.PodLogOptions{Container: container.Name, Timestamps: true}

					if ls.lastChecked[curContainerId] != nil {
						podLogOpts.SinceTime = ls.lastChecked[curContainerId]
					}

					log, err := ls.clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts).Do(ctx).Raw()
					if err != nil {
						fmt.Println(errors.Wrap(err, "error reading container log"))
						continue
					}

					times, messages, err := ls.splitTimestampsFromMessages(log, curContainerId)
					if err != nil {
						return err
					}

					if len(messages) == 0 {
						continue
					}

					newLog := &schema.Log{
						Id:            curContainerId[:],
						ReferenceId:   curPodId[:],
						ContainerName: container.Name,
						Time:          strings.Join(times, "\n"),
						Log:           strings.Join(messages, "\n"),
					}

					upserts <- newLog

					lastTime, err := time.Parse("2006-01-02T15:04:05.999999999Z", times[len(times)-1])
					if err != nil {
						return errors.Wrap(err, "error parsing log time")
					}

					if !slices.Contains(ls.list, pod) {
						continue
					}

					lastV1Time := kmetav1.Time{Time: lastTime}
					ls.lastChecked[curContainerId] = &lastV1Time
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
		return ls.db.UpsertStreamed(ctx, upserts, database.WithStatement(upsertStmt, 5))
	})

	return g.Wait()
}
