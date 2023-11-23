package sync

import (
	"context"
	"crypto/sha1"
	"github.com/icinga/icinga-go-library/database"
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
}

func NewMetricSync(metricsClientset *metricsv.Clientset, db *database.DB) *MetricSync {
	return &MetricSync{
		metricsClientset: metricsClientset,
		db:               db,
	}
}

func (ms *MetricSync) Run(ctx context.Context) error {

	g, ctx := errgroup.WithContext(ctx)

	upsertPodMetrics := make(chan database.Entity)
	upsertContainerMetrics := make(chan database.Entity)
	defer close(upsertPodMetrics)
	defer close(upsertContainerMetrics)

	g.Go(func() error {
		for {
			metrics, err := ms.metricsClientset.MetricsV1beta1().PodMetricses(kmetav1.NamespaceAll).List(ctx, kmetav1.ListOptions{})
			if err != nil {
				return errors.Wrap(err, "error getting metrics from api")
			}

			for _, pod := range metrics.Items {

				podId := sha1.Sum([]byte(pod.Namespace + "/" + pod.Name))

				newPodMetric := &schema.PodMetric{}
				//
				//newPodMetric := schema.NewPodMetric(
				//	podId[:],
				//	pod.Timestamp.UnixMilli(),
				//	0,
				//	0,
				//	0,
				//)

				for _, container := range pod.Containers {

					containerId := sha1.Sum([]byte(pod.Namespace + "/" + pod.Name + "/" + container.Name))

					newContainerMetric := schema.NewContainerMetric(
						containerId[:],
						podId[:],
						pod.Timestamp.UnixMilli(),
						container.Usage.Cpu().MilliValue(),
						container.Usage.Memory().Value(),
						container.Usage.Storage().Value(),
					)

					upsertContainerMetrics <- newContainerMetric

					newPodMetric.IncreaseCpu(container.Usage.Cpu().MilliValue())
					newPodMetric.IncreaseMemory(container.Usage.Memory().Value())
					newPodMetric.IncreaseStorage(container.Usage.Storage().Value())
				}

				upsertPodMetrics <- newPodMetric
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Minute):
			}
		}
	})

	//g.Go(func() error {
	//	return ...
	//})
	//
	//g.Go(func() error {
	//	return ...
	//})

	return g.Wait()
}
