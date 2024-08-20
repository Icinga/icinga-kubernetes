package metrics

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/periodic"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/pkg/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	kcorev1 "k8s.io/api/core/v1"
	kcache "k8s.io/client-go/tools/cache"
	"net"
	"strings"
	"sync"
	"time"
)

// PromQuery defines a prometheus query with the metric group, the query and the name label
type PromQuery struct {
	metricCategory string
	query          string
	nameLabel      model.LabelName
}

var (
	promQueriesCluster = []PromQuery{
		{
			"node.count",
			`count(group by (node) (kube_node_info))`,
			"",
		},
		{
			"namespace.count",
			`count(kube_namespace_created)`,
			"",
		},
		{
			"pod.running",
			`sum(kube_pod_status_phase{phase="Running"})`,
			"",
		},
		{
			"pod.pending",
			`sum(kube_pod_status_phase{phase="Pending"})`,
			"",
		},
		{
			"pod.failed",
			`sum(kube_pod_status_phase{phase="Failed"})`,
			"",
		},
		{
			"pod.succeeded",
			`sum(kube_pod_status_phase{phase="Succeeded"})`,
			"",
		},
		{
			"cpu.usage",
			`avg(sum by (instance, cpu) (rate(node_cpu_seconds_total{mode!~"idle|iowait|steal"}[1m])))`,
			"",
		},
		{
			"memory.usage",
			`sum(node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes)`,
			"",
		},
		{
			"qos_by_class",
			`sum by (qos_class) (kube_pod_status_qos_class)`,
			"",
		},
		{
			"network.received.bytes",
			`sum by (device) (rate(node_network_receive_bytes_total{device!~"(veth|azv|lxc).*"}[2m]))`,
			"",
		},
		{
			"network.transmitted.bytes",
			`- sum by (device) (rate(node_network_transmit_bytes_total{device!~"(veth|azv|lxc).*"}[2m]))`,
			"",
		},
		{
			"network.received.bytes.bydevice",
			`sum by (device) (rate(node_network_receive_bytes_total{device!~"(veth|azv|lxc).*"}[2m]))`,
			"device",
		},
	}

	promQueriesNode = []PromQuery{
		{
			"cpu.usage",
			`avg by (node) (sum by (node, cpu) (rate(node_cpu_seconds_total{mode!~"idle|iowait|steal"}[2m])))`,
			// TODO(el): Check this alternative.
			//`avg without (mode,cpu) (1 - rate(node_cpu_seconds_total{mode="idle"}[1m]))`,
			"",
		},
		{
			"cpu.request",
			`sum by (node) (kube_pod_container_resource_requests{resource="cpu"})`,
			"",
		},
		{
			"cpu.request.percentage",
			`sum by (node) (kube_pod_container_resource_requests{resource="cpu"}) / on(node) group_left() (sum by (node) (machine_cpu_cores))`,
			"",
		},
		{
			"cpu.limit",
			`sum by (node) (kube_pod_container_resource_limits{resource="cpu"})`,
			"",
		},
		{
			"cpu.limit.percentage",
			`sum by (node) (kube_pod_container_resource_limits{resource="cpu"}) / on(node) group_left() (sum by (node) (machine_cpu_cores))`,
			"",
		},
		{
			"memory.usage",
			`sum by (instance) (node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) / sum by (instance) (node_memory_MemTotal_bytes)`,
			"",
		},
		{
			"memory.request",
			`sum by (node) (kube_pod_container_resource_requests{resource="memory"})`,
			"",
		},
		{
			"memory.request.percentage",
			`sum by (node) (kube_pod_container_resource_requests{resource="memory"}) / on(node) group_left() (sum by (node) (machine_memory_bytes))`,
			"",
		},
		{
			"memory.limit",
			`sum by (node) (kube_pod_container_resource_limits{resource="memory"})`,
			"",
		},
		{
			"memory.limit.percentage",
			`sum by (node) (kube_pod_container_resource_limits{resource="memory"}) / on(node) group_left() (sum by (node) (machine_memory_bytes))`,
			"",
		},
		{
			"network.received.bytes",
			`sum by (node) (rate(node_network_receive_bytes_total[2m]))`,
			"",
		},
		{
			"network.transmitted.bytes",
			`- sum by (node) (rate(node_network_transmit_bytes_total[2m]))`,
			"",
		},
		{
			"filesystem.usage",
			`sum by (node, mountpoint) (1 - (node_filesystem_avail_bytes / node_filesystem_size_bytes))`,
			"mountpoint",
		},
	}

	promQueriesPod = []PromQuery{
		{
			"cpu.usage",
			`sum by (instance, namespace, pod) (rate(container_cpu_usage_seconds_total[2m]))`,
			"",
		},
		{
			"memory.usage",
			`sum by (instance, namespace, pod) (container_memory_usage_bytes) / on () group_left() label_replace(node_memory_MemTotal_bytes, "instance", "$1", "node", "(.*)")`,
			"",
		},
		{
			"cpu.usage.cores",
			`sum by (namespace, pod) (rate(container_cpu_usage_seconds_total[2m]))`,
			"",
		},
		{
			"memory.usage.bytes",
			`sum by (namespace, pod) (container_memory_usage_bytes)`,
			"",
		},
		{
			"cpu.request",
			`sum by (node, namespace, pod) (kube_pod_container_resource_requests{resource="cpu"})`,
			"",
		},
		{
			"cpu.request.percentage",
			`sum by (node, namespace, pod) (kube_pod_container_resource_requests{resource="cpu"}) / on(node) group_left() (sum by (node) (machine_cpu_cores))`,
			"",
		},
		{
			"cpu.limit",
			`sum by (node, namespace, pod) (kube_pod_container_resource_limits{resource="cpu"})`,
			"",
		},
		{
			"cpu.limit.percentage",
			`sum by (node, namespace, pod) (kube_pod_container_resource_limits{resource="cpu"}) / on(node) group_left() (sum by (node) (machine_cpu_cores))`,
			"",
		},
		{
			"memory.request",
			`sum by (node, namespace, pod) (kube_pod_container_resource_requests{resource="memory"})`,
			"",
		},
		{
			"memory.request.percentage",
			`sum by (node, namespace, pod) (kube_pod_container_resource_requests{resource="memory"}) / on(node) group_left() (sum by (node) (machine_memory_bytes))`,
			"",
		},
		{
			"memory.limit",
			`sum by (node, namespace, pod) (kube_pod_container_resource_limits{resource="memory"})`,
			"",
		},
		{
			"memory.limit.percentage",
			`sum by (node, namespace, pod) (kube_pod_container_resource_limits{resource="memory"}) / on(node) group_left() (sum by (node) (machine_memory_bytes))`,
			"",
		},
	}

	promQueriesContainer = []PromQuery{
		{
			"cpu.request",
			`sum by (node, namespace, pod, container) (kube_pod_container_resource_requests{resource="cpu"})`,
			"",
		},
		{
			"cpu.request.percentage",
			`sum by (node, namespace, pod, container) (kube_pod_container_resource_requests{resource="cpu"}) / on(node) group_left() (sum by (node) (machine_cpu_cores))`,
			"",
		},
		{
			"cpu.limit",
			`sum by (node, namespace, pod, container) (kube_pod_container_resource_limits{resource="cpu"})`,
			"",
		},
		{
			"cpu.limit.percentage",
			`sum by (node, namespace, pod, container) (kube_pod_container_resource_limits{resource="cpu"}) / on(node) group_left() (sum by (node) (machine_cpu_cores))`,
			"",
		},
		{
			"memory.request",
			`sum by (node, namespace, pod, container) (kube_pod_container_resource_requests{resource="memory"})`,
			"",
		},
		{
			"memory.request.percentage",
			`sum by (node, namespace, pod, container) (kube_pod_container_resource_requests{resource="memory"}) / on(node) group_left() (sum by (node) (machine_memory_bytes))`,
			"",
		},
		{
			"memory.limit",
			`sum by (node, namespace, pod, container) (kube_pod_container_resource_limits{resource="memory"})`,
			"",
		},
		{
			"memory.limit.percentage",
			`sum by (node, namespace, pod, container) (kube_pod_container_resource_limits{resource="memory"}) / on(node) group_left() (sum by (node) (machine_memory_bytes))`,
			"",
		},
	}
)

// PromMetricSync synchronizes prometheus metrics from the prometheus API to the database
type PromMetricSync struct {
	promApiClient v1.API
	db            *database.DB
	logger        *logging.Logger
}

// NewPromMetricSync creates a new PromMetricSync
func NewPromMetricSync(promApiClient v1.API, db *database.DB, logger *logging.Logger) *PromMetricSync {
	return &PromMetricSync{
		promApiClient: promApiClient,
		db:            db,
		logger:        logger,
	}
}

// promMetricClusterUpsertStmt returns database upsert statement to upsert cluster metrics
func (pms *PromMetricSync) promMetricClusterUpsertStmt() string {
	return fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s`,
		`prometheus_cluster_metric`,
		"cluster_id, timestamp, category, name, value",
		`:cluster_id, :timestamp, :category, :name, :value`,
		`value=VALUES(value)`,
	)
}

// promMetricNodeUpsertStmt returns database upsert statement to upsert node metrics
func (pms *PromMetricSync) promMetricNodeUpsertStmt() string {
	return fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s`,
		`prometheus_node_metric`,
		"node_uuid, timestamp, category, name, value",
		`:node_uuid, :timestamp, :category, :name, :value`,
		`value=VALUES(value)`,
	)
}

// promMetricPodUpsertStmt returns database upsert statement to upsert pod metrics
func (pms *PromMetricSync) promMetricPodUpsertStmt() string {
	return fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s`,
		`prometheus_pod_metric`,
		"pod_uuid, timestamp, category, name, value",
		`:pod_uuid, :timestamp, :category, :name, :value`,
		`value=VALUES(value)`,
	)
}

// promMetricContainerUpsertStmt returns database upsert statement to upsert container metrics
func (pms *PromMetricSync) promMetricContainerUpsertStmt() string {
	return fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s`,
		`prometheus_container_metric`,
		"container_id, timestamp, category, name, value",
		`:container_id, :timestamp, :category, :name, :value`,
		`value=VALUES(value)`,
	)
}

func (pms *PromMetricSync) run(
	ctx context.Context,
	promQueries []PromQuery,
	upsertMetrics chan<- database.Entity,
	getEntity func(query PromQuery, res *model.Sample) database.Entity,
) error {
	g, ctx := errgroup.WithContext(ctx)

	for _, promQuery := range promQueries {
		promQuery := promQuery

		g.Go(func() error {
			var result model.Value
			var warnings v1.Warnings
			var err error

			for {
				err := retry.WithBackoff(
					ctx,
					func(ctx context.Context) error {
						result, warnings, err = pms.promApiClient.Query(
							ctx,
							promQuery.query,
							time.Time{},
						)

						return err
					},
					retry.Retryable,
					backoff.NewExponentialWithJitter(1*time.Millisecond, 1*time.Second),
					retry.Settings{
						Timeout: retry.DefaultTimeout,
						OnRetryableError: func(_ time.Duration, _ uint64, err, lastErr error) {
							if lastErr == nil || err.Error() != lastErr.Error() {
								pms.logger.Warnw("Can't execute prometheus query. Retrying", zap.Error(err))
							}
						},
						OnSuccess: func(elapsed time.Duration, attempt uint64, lastErr error) {
							if attempt > 1 {
								pms.logger.Infow("Query retried successfully after error",
									zap.Duration("after", elapsed),
									zap.Uint64("attempts", attempt),
									zap.NamedError("recovered_error", lastErr))
							}
						},
					},
				)
				if err != nil {
					return errors.Wrap(err, "error querying Prometheus")
				}

				if len(warnings) > 0 {
					pms.logger.Warnf("Prometheus warnings: %v\n", warnings)
				}
				if result == nil {
					continue
				}
				for _, res := range result.(model.Vector) {
					entity := getEntity(promQuery, res)
					if entity == nil {
						continue
					}

					select {
					case upsertMetrics <- entity:
					case <-ctx.Done():
						return ctx.Err()
					}
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Second * 60):
				}
			}
		})
	}

	return g.Wait()
}

func (pms *PromMetricSync) Nodes(ctx context.Context, informer kcache.SharedIndexInformer) error {
	if !kcache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return errors.New("timed out waiting for caches to sync")
	}

	nodes := sync.Map{}
	defer periodic.Start(ctx, 1*time.Hour, func(tick periodic.Tick) {
		for _, item := range informer.GetStore().List() {
			node := item.(*kcorev1.Node)
			uuid := schemav1.EnsureUUID(node.UID)
			nodes.Store(node.Name, uuid)
			for _, address := range node.Status.Addresses {
				if address.Type == kcorev1.NodeInternalIP {
					nodes.Store(address.Address, uuid)
				}
			}
		}
	}, periodic.Immediate()).Stop()

	upsertMetrics := make(chan database.Entity)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return pms.run(
			ctx,
			promQueriesNode,
			upsertMetrics,
			func(query PromQuery, res *model.Sample) database.Entity {
				if res.Value.String() == "NaN" {
					return nil
				}

				nodeName := string(res.Metric["node"])
				if nodeName == "" {
					if strings.Contains(string(res.Metric["instance"]), ":") {
						if host, _, err := net.SplitHostPort(string(res.Metric["instance"])); err == nil {
							nodeName = host
						} else {
							return nil
						}
					} else {
						nodeName = string(res.Metric["instance"])
					}
				}
				uuid, exists := nodes.Load(nodeName)
				if !exists {
					return nil
				}

				name := ""
				if query.nameLabel != "" {
					name = string(res.Metric[query.nameLabel])
				}

				newNodeMetric := &schemav1.PrometheusNodeMetric{
					NodeUuid:  uuid.(types.UUID),
					Timestamp: (res.Timestamp.UnixNano() - res.Timestamp.UnixNano()%(60*1000000000)) / 1000000,
					Category:  query.metricCategory,
					Name:      name,
					Value:     float64(res.Value),
				}

				return newNodeMetric
			},
		)
	})

	g.Go(func() error {
		return database.NewUpsert(pms.db, database.WithStatement(pms.promMetricNodeUpsertStmt(), 5)).Stream(ctx, upsertMetrics)
	})

	return g.Wait()
}

func (pms *PromMetricSync) Pods(ctx context.Context, informer kcache.SharedIndexInformer) error {
	if !kcache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return errors.New("timed out waiting for caches to sync")
	}

	upsertMetrics := make(chan database.Entity)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return pms.run(
			ctx,
			promQueriesPod,
			upsertMetrics,
			func(query PromQuery, res *model.Sample) database.Entity {
				if res.Metric["pod"] == "" {
					return nil
				}

				obj, exists, err := informer.GetStore().GetByKey(
					kcache.NewObjectName(string(res.Metric["namespace"]), string(res.Metric["pod"])).String())
				if err != nil {
					//return errors.Wrap(err, "can't get pod from store")
					return nil
				}
				if !exists {
					return nil
				}
				pod := obj.(*kcorev1.Pod)

				name := ""
				if query.nameLabel != "" {
					name = string(res.Metric[query.nameLabel])
				}

				newPodMetric := &schemav1.PrometheusPodMetric{
					PodUuid:   schemav1.EnsureUUID(pod.UID),
					Timestamp: (res.Timestamp.UnixNano() - res.Timestamp.UnixNano()%(60*1000000000)) / 1000000,
					Category:  query.metricCategory,
					Name:      name,
					Value:     float64(res.Value),
				}

				return newPodMetric
			},
		)
	})

	g.Go(func() error {
		return database.NewUpsert(pms.db, database.WithStatement(pms.promMetricPodUpsertStmt(), 5)).Stream(ctx, upsertMetrics)
	})

	return g.Wait()
}

func (pms *PromMetricSync) Containers(ctx context.Context, informer kcache.SharedIndexInformer) error {
	if !kcache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		pms.logger.Fatal("timed out waiting for caches to sync")
	}

	upsertMetrics := make(chan database.Entity)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return pms.run(
			ctx,
			promQueriesContainer,
			upsertMetrics,
			func(query PromQuery, res *model.Sample) database.Entity {
				if res.Value.String() == "NaN" {
					return nil
				}

				//containerId := sha1.Sum([]byte(res.Metric["namespace"] + "/" + res.Metric["pod"] + "/" + res.Metric["container"]))

				name := ""

				if query.nameLabel != "" {
					name = string(res.Metric[query.nameLabel])
				}

				newContainerMetric := &schemav1.PrometheusContainerMetric{
					// TODO uuid
					Timestamp: (res.Timestamp.UnixNano() - res.Timestamp.UnixNano()%(60*1000000000)) / 1000000,
					Category:  query.metricCategory,
					Name:      name,
					Value:     float64(res.Value),
				}

				return newContainerMetric
			},
		)
	})

	g.Go(func() error {
		return database.NewUpsert(pms.db, database.WithStatement(pms.promMetricContainerUpsertStmt(), 5)).Stream(ctx, upsertMetrics)
	})

	return g.Wait()
}

func (pms *PromMetricSync) Clusters(ctx context.Context, informer kcache.SharedIndexInformer) error {
	if !kcache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		pms.logger.Fatal("timed out waiting for caches to sync")
	}

	upsertMetrics := make(chan database.Entity)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return pms.run(
			ctx,
			promQueriesCluster,
			upsertMetrics,
			func(query PromQuery, res *model.Sample) database.Entity {
				if res.Value.String() == "NaN" {
					return nil
				}

				//clusterId := sha1.Sum([]byte(""))

				name := ""

				if query.nameLabel != "" {
					name = string(res.Metric[query.nameLabel])
				}

				newClusterMetric := &schemav1.PrometheusClusterMetric{
					// TODO uuid
					Timestamp: (res.Timestamp.UnixNano() - res.Timestamp.UnixNano()%(60*1000000000)) / 1000000,
					Category:  query.metricCategory,
					Name:      name,
					Value:     float64(res.Value),
				}

				return newClusterMetric
			},
		)
	})

	g.Go(func() error {
		return database.NewUpsert(pms.db, database.WithStatement(pms.promMetricClusterUpsertStmt(), 5)).Stream(ctx, upsertMetrics)
	})

	return g.Wait()
}
