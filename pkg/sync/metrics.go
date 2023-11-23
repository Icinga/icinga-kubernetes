package sync

import (
	"context"
	"crypto/sha1"
	"fmt"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/schema"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	//kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

type MetricSync struct {
	metricsClientset *metricsv.Clientset
	db               *database.DB
	logger           *logging.Logger
}

func NewMetricSync(metricsClientset *metricsv.Clientset, db *database.DB, logger *logging.Logger) *MetricSync {
	return &MetricSync{
		metricsClientset: metricsClientset,
		db:               db,
		logger:           logger,
	}
}

func (ms *MetricSync) Run(ctx context.Context) error {

	ms.logger.Info("Starting sync")

	g, ctx := errgroup.WithContext(ctx)

	upsertPodMetrics := make(chan database.Entity)
	upsertContainerMetrics := make(chan database.Entity)

	g.Go(func() error {
		defer close(upsertPodMetrics)
		defer close(upsertContainerMetrics)

		for {
			metrics, err := ms.metricsClientset.MetricsV1beta1().PodMetricses(kmetav1.NamespaceAll).List(ctx, kmetav1.ListOptions{})
			if err != nil {
				return errors.Wrap(err, "error getting metrics from api")
			}

			for _, pod := range metrics.Items {

				podId := sha1.Sum([]byte(pod.Namespace + "/" + pod.Name))

				newPodMetric := &schema.PodMetric{
					ReferenceId: podId[:],
					Timestamp:   pod.Timestamp.UnixMilli(),
				}

				for _, container := range pod.Containers {

					containerId := sha1.Sum([]byte(pod.Namespace + "/" + pod.Name + "/" + container.Name))

					newContainerMetric := &schema.ContainerMetric{
						ContainerReferenceId: containerId[:],
						PodReferenceId:       podId[:],
						Timestamp:            pod.Timestamp.UnixMilli(),
						Cpu:                  container.Usage.Cpu().MilliValue(),
						Memory:               container.Usage.Memory().Value(),
						Storage:              container.Usage.Storage().Value(),
					}

					upsertContainerMetrics <- newContainerMetric

					newPodMetric.Cpu += container.Usage.Cpu().MilliValue()
					newPodMetric.Memory += container.Usage.Memory().Value()
					newPodMetric.Storage += container.Usage.Storage().Value()
				}

				upsertPodMetrics <- newPodMetric
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second * 5):
				//case <-time.After(time.Minute):
			}
		}
	})

	g.Go(func() error {
		return ms.db.UpsertStreamedWithStatement(ctx, upsertPodMetrics, ms.podMetricUpsertStmt(), 5)
	})

	g.Go(func() error {
		return ms.db.UpsertStreamedWithStatement(ctx, upsertContainerMetrics, ms.containerMetricUpsertStmt(), 6)
	})

	return g.Wait()
}

func (ms *MetricSync) Clean(ctx context.Context, deleteChannel <-chan contracts.KDelete) error {

	g, ctx := errgroup.WithContext(ctx)

	deletesPod := make(chan any)
	deletesContainer := make(chan any)

	g.Go(func() error {
		defer close(deletesPod)
		defer close(deletesContainer)

		for {
			select {
			case kdelete, more := <-deleteChannel:
				if !more {
					return nil
				}

				deletesPod <- kdelete.ID()
				deletesContainer <- kdelete.ID()

			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	g.Go(func() error {
		return ms.db.DeleteStreamedByField(ctx, &schema.PodMetric{}, "reference_id", deletesPod)
	})

	g.Go(func() error {
		return ms.db.DeleteStreamedByField(ctx, &schema.ContainerMetric{}, "pod_reference_id", deletesContainer)
	})

	return g.Wait()
}

func (ms *MetricSync) podMetricUpsertStmt() string {
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		"pod_metric",
		"reference_id, timestamp, cpu, memory, storage",
		":reference_id, :timestamp, :cpu, :memory, :storage",
		"timestamp=VALUES(timestamp), cpu=VALUES(cpu), memory=VALUES(memory), storage=VALUES(storage)",
	)
}

func (ms *MetricSync) containerMetricUpsertStmt() string {
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		"container_metric",
		"container_reference_id, pod_reference_id, timestamp, cpu, memory, storage",
		":container_reference_id, :pod_reference_id, :timestamp, :cpu, :memory, :storage",
		"timestamp=VALUES(timestamp), cpu=VALUES(cpu), memory=VALUES(memory), storage=VALUES(storage)",
	)
}

type NodeMetricSync struct {
	metricsClientset *metricsv.Clientset
	db               *database.DB
	logger           *logging.Logger
}

func NewNodeMetricSync(metricClientset *metricsv.Clientset, db *database.DB, logger *logging.Logger) *NodeMetricSync {
	return &NodeMetricSync{
		metricsClientset: metricClientset,
		db:               db,
		logger:           logger,
	}
}

func (nms *NodeMetricSync) Run(ctx context.Context) error {

	g, ctx := errgroup.WithContext(ctx)

	upsertNodeMetrics := make(chan database.Entity)

	g.Go(func() error {

		defer close(upsertNodeMetrics)

		for {
			metrics, err := nms.metricsClientset.MetricsV1beta1().NodeMetricses().List(ctx, kmetav1.ListOptions{})
			if err != nil {
				return errors.Wrap(err, "error getting node metrics from api")
			}

			for _, node := range metrics.Items {
				nodeId := sha1.Sum([]byte(node.Name))

				newNodeMetric := &schema.NodeMetric{
					NodeId:    nodeId[:],
					Timestamp: node.Timestamp.UnixMilli(),
					Cpu:       node.Usage.Cpu().MilliValue(),
					Memory:    node.Usage.Memory().Value(),
					Storage:   node.Usage.Storage().Value(),
				}

				upsertNodeMetrics <- newNodeMetric
			}
		}
	})

	g.Go(func() error {
		return nms.db.UpsertStreamedWithStatement(ctx, upsertNodeMetrics, nms.nodeMetricUpsertStmt(), 5)
	})

	return g.Wait()
}

func (nms *NodeMetricSync) Clean(ctx context.Context, deleteChannel <-chan contracts.KDelete) error {

	g, ctx := errgroup.WithContext(ctx)

	deletes := make(chan any)

	g.Go(func() error {
		defer close(deletes)

		for {
			select {
			case kdelete, more := <-deleteChannel:
				if !more {
					return nil
				}

				deletes <- kdelete.ID()

			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	g.Go(func() error {
		return nms.db.DeleteStreamedByField(ctx, &schema.NodeMetric{}, "node_id", deletes)
	})

	return g.Wait()
}

func (nms *NodeMetricSync) nodeMetricUpsertStmt() string {
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		"node_metric",
		"node_id, timestamp, cpu, memory, storage",
		":node_id, :timestamp, :cpu, :memory, :storage",
		"timestamp=VALUES(timestamp), cpu=VALUES(cpu), memory=VALUES(memory), storage=VALUES(storage)",
	)
}
