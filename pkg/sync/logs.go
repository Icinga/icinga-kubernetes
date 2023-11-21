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
	v1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"slices"
	"strings"
	msync "sync"
	"time"
)

type containerId [20]byte

type LogSync struct {
	list        []*v1.Pod
	lastChecked map[containerId]*kmetav1.Time
	mutex       *msync.RWMutex
	clientset   *kubernetes.Clientset
	db          *database.DB
	logger      *logging.Logger
}

func NewLogSync(clientset *kubernetes.Clientset, db *database.DB, logger *logging.Logger) *LogSync {
	return &LogSync{
		list:        []*v1.Pod{},
		lastChecked: make(map[containerId]*kmetav1.Time),
		mutex:       &msync.RWMutex{},
		clientset:   clientset,
		db:          db,
		logger:      logger,
	}
}

func (ls *LogSync) splitTimestampsFromMessages(log []byte, curContainerId containerId) (times []string, messages []string, err error) {

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

func (ls *LogSync) removeFromList(id database.ID) {
	out := make([]*v1.Pod, 0)

	for _, element := range ls.list {

		elementId := sha1.Sum([]byte(element.Namespace + "/" + element.Name))

		if fmt.Sprintf("%x", elementId) != id.String() {
			out = append(out, element)
		}
	}

	ls.list = out
}

func (ls *LogSync) MaintainList(ctx context.Context, addChannel <-chan database.Entity, deleteChannel <-chan any) error {

	ls.logger.Info("Starting maintain list")

	g, ctx := errgroup.WithContext(ctx)

	deletes := make(chan any)
	defer close(deletes)

	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return errors.Wrap(ctx.Err(), "context canceled maintain log sync list")

			case podFromChannel, more := <-addChannel:
				if !more {
					return nil
				}

				podEntity, ok := podFromChannel.(contracts.Resource)
				if !ok {
					continue
				}

				pod, err := ls.clientset.CoreV1().Pods(podEntity.GetNamespace()).Get(ctx, podEntity.GetName(), kmetav1.GetOptions{})
				if err != nil {
					continue
					return errors.Wrap(err, "error getting pod for maintaining log sync list")
				}

				nextLoop := false

				for _, listPod := range ls.list {
					if listPod.UID == pod.UID {
						nextLoop = true
					}
				}

				if nextLoop {
					continue
				}

				ls.mutex.RLock()
				ls.list = append(ls.list, pod)
				ls.mutex.RUnlock()

			case podIdFromChannel, more := <-deleteChannel:
				if !more {
					return nil
				}

				idOfPod := podIdFromChannel.(database.ID)

				ls.mutex.RLock()
				ls.removeFromList(idOfPod)
				ls.mutex.RUnlock()

				deletes <- idOfPod

				//_, err := deleteStmt.ExecContext(ctx, idOfPod)
				//if err != nil {
				//	return errors.Wrap(err, "error executing delete stmt for log")
				//}
			}

		}
	})

	g.Go(func() error {
		return ls.db.DeleteStreamedByField(ctx, schema.NewLog(), "reference_id", deletes)
	})

	return g.Wait()
}

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

					podLogOpts := v1.PodLogOptions{Container: container.Name, Timestamps: true}

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

					//_, err = upsertStmt.ExecContext(ctx, schema.Log{
					//	Id:            curContainerId[:],
					//	ReferenceId:   curPodId[:],
					//	ContainerName: container.Name,
					//	Time:          strings.Join(times, "\n"),
					//	Log:           strings.Join(messages, "\n"),
					//})
					//if err != nil {
					//	return errors.Wrap(err, "error executing upsert stmt for log")
					//}

					newLog := schema.NewLog(schema.WithValues(curContainerId[:], curPodId[:], container.Name, strings.Join(times, "\n"), strings.Join(messages, "\n")))

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
				return errors.Wrap(ctx.Err(), "context canceled run log sync")
			case <-time.After(time.Second * 5):
			}
		}
	})

	g.Go(func() error {
		return ls.db.UpsertStreamedWithStatement(ctx, upserts, upsertStmt, 5)
	})

	return g.Wait()
}

func (ls *LogSync) upsertStmt() string {
	return "INSERT INTO log (id, reference_id, container_name, time, log) VALUES (:id, :reference_id, :container_name, :time, :log) ON DUPLICATE KEY UPDATE time=CONCAT(time, '\n', :time), log=CONCAT(log, '\n', :log)"
}
