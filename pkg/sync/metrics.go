package sync

import (
	"context"
	"crypto/sha1"
	"fmt"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-kubernetes/pkg/schema"
	"github.com/pkg/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"golang.org/x/sync/errgroup"
	"time"
)

type PromQuery struct {
	metricGroup string
	query       string
	nameLabel   model.LabelName
}

type PromMetricSync struct {
	promApiClient v1.API
	db            *database.DB
	logger        *logging.Logger
}

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
		"timestamp, `group`, name, value",
		`:timestamp, :group, :name, :value`,
		`value=VALUES(value)`,
	)
}

// promMetricNodeUpsertStmt returns database upsert statement to upsert node metrics
func (pms *PromMetricSync) promMetricNodeUpsertStmt() string {
	return fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s`,
		`prometheus_node_metric`,
		"node_id, timestamp, `group`, name, value",
		`:node_id, :timestamp, :group, :name, :value`,
		`value=VALUES(value)`,
	)
}

// promMetricPodUpsertStmt returns database upsert statement to upsert pod metrics
func (pms *PromMetricSync) promMetricPodUpsertStmt() string {
	return fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s`,
		`prometheus_pod_metric`,
		"pod_id, timestamp, `group`, name, value",
		`:pod_id, :timestamp, :group, :name, :value`,
		`value=VALUES(value)`,
	)
}

// promMetricContainerUpsertStmt returns database upsert statement to upsert container metrics
func (pms *PromMetricSync) promMetricContainerUpsertStmt() string {
	return fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s`,
		`prometheus_container_metric`,
		"container_id, timestamp, `group`, name, value",
		`:container_id, :timestamp, :group, :name, :value`,
		`value=VALUES(value)`,
	)
}

// Run starts syncing the prometheus metrics to the database.
// Therefore, it gets a list of the metric queries.
func (pms *PromMetricSync) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	upsertClusterMetrics := make(chan database.Entity)
	upsertNodeMetrics := make(chan database.Entity)
	upsertPodMetrics := make(chan database.Entity)
	upsertContainerMetrics := make(chan database.Entity)

	promQueriesCluster := []PromQuery{
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

	promQueriesNode := []PromQuery{
		{
			"cpu.usage",
			`avg by (instance) (sum by (instance, cpu) (rate(node_cpu_seconds_total{mode!~"idle|iowait|steal"}[1m])))`,
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
			`sum by (instance) (rate(node_network_receive_bytes_total[2m]))`,
			"",
		},
		{
			"network.transmitted.bytes",
			`- sum by (instance) (rate(node_network_transmit_bytes_total[2m]))`,
			"",
		},
		{
			"filesystem.usage",
			`sum by (instance, mountpoint) (1 - (node_filesystem_avail_bytes / node_filesystem_size_bytes))`,
			"mountpoint",
		},
	}

	promQueriesPod := []PromQuery{
		{
			"cpu.usage",
			`avg by (namespace, pod) (sum by (namespace, pod, cpu) (rate(node_cpu_seconds_total{mode!~"idle|iowait|steal"}[1m])))`,
			"",
		},
		{
			"memory.usage",
			`sum by (namespace, pod) ((node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes))`,
			"",
		},
		{
			"cpu.usage.cores",
			`sum by (namespace, pod) (rate(container_cpu_usage_seconds_total[1m]))`,
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

	promQueriesContainer := []PromQuery{
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

	//promv1.Range{
	//	Start: time.Now().Add(time.Duration(-2) * time.Hour),
	//	End:   time.Now(),
	//	Step:  time.Second * 10,
	//},

	for _, promQuery := range promQueriesCluster {
		promQuery := promQuery

		g.Go(func() error {
			for {
				result, warnings, err := pms.promApiClient.Query(
					ctx,
					promQuery.query,
					time.Time{},
					//promQuery.queryRange,
				)
				if err != nil {
					return errors.Wrap(err, "error querying Prometheus")
				}
				if len(warnings) > 0 {
					fmt.Printf("Warnings: %v\n", warnings)
				}
				if result == nil {
					fmt.Println("No results found")
					continue
				}

				for _, res := range result.(model.Vector) {

					name := ""

					if promQuery.nameLabel != "" {
						name = string(res.Metric[promQuery.nameLabel])
					}

					newClusterMetric := &schema.PrometheusClusterMetric{
						Timestamp: (res.Timestamp.UnixNano() - res.Timestamp.UnixNano()%(60*1000000000)) / 1000000,
						Group:     promQuery.metricGroup,
						Name:      name,
						Value:     float64(res.Value),
					}

					select {
					case upsertClusterMetrics <- newClusterMetric:
					case <-ctx.Done():
						return ctx.Err()
					}
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Second * 55):
				}
			}
		})
	}

	for _, promQuery := range promQueriesNode {
		promQuery := promQuery

		g.Go(func() error {
			for {
				result, warnings, err := pms.promApiClient.Query(
					ctx,
					promQuery.query,
					time.Time{},
					//promQuery.queryRange,
				)
				if err != nil {
					return errors.Wrap(err, "error querying Prometheus")
				}
				if len(warnings) > 0 {
					fmt.Printf("Warnings: %v\n", warnings)
				}
				if result == nil {
					fmt.Println("No results found")
					continue
				}

				for _, res := range result.(model.Vector) {
					nodeName := res.Metric["node"]

					if nodeName == "" {
						nodeName = res.Metric["instance"]
					}

					nodeId := sha1.Sum([]byte(nodeName))

					name := ""

					if promQuery.nameLabel != "" {
						name = string(res.Metric[promQuery.nameLabel])
					}

					newNodeMetric := &schema.PrometheusNodeMetric{
						NodeId:    nodeId[:],
						Timestamp: (res.Timestamp.UnixNano() - res.Timestamp.UnixNano()%(60*1000000000)) / 1000000,
						Group:     promQuery.metricGroup,
						Name:      name,
						Value:     float64(res.Value),
					}

					select {
					case upsertNodeMetrics <- newNodeMetric:
					case <-ctx.Done():
						return ctx.Err()
					}
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Second * 55):
				}
			}
		})
	}

	for _, promQuery := range promQueriesPod {
		promQuery := promQuery

		g.Go(func() error {
			for {
				result, warnings, err := pms.promApiClient.Query(
					ctx,
					promQuery.query,
					time.Time{},
					//promQuery.queryRange,
				)
				if err != nil {
					return errors.Wrap(err, "error querying Prometheus")
				}
				if len(warnings) > 0 {
					fmt.Printf("Warnings: %v\n", warnings)
				}
				if result == nil {
					fmt.Println("No results found")
					continue
				}

				for _, res := range result.(model.Vector) {

					podId := sha1.Sum([]byte(res.Metric["namespace"] + "/" + res.Metric["pod"]))

					name := ""

					if promQuery.nameLabel != "" {
						name = string(res.Metric[promQuery.nameLabel])
					}

					newPodMetric := &schema.PrometheusPodMetric{
						PodId:     podId[:],
						Timestamp: (res.Timestamp.UnixNano() - res.Timestamp.UnixNano()%(60*1000000000)) / 1000000,
						Group:     promQuery.metricGroup,
						Name:      name,
						Value:     float64(res.Value),
					}

					select {
					case upsertPodMetrics <- newPodMetric:
					case <-ctx.Done():
						return ctx.Err()
					}
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Second * 55):
				}
			}
		})
	}

	for _, promQuery := range promQueriesContainer {
		promQuery := promQuery

		g.Go(func() error {
			for {
				result, warnings, err := pms.promApiClient.Query(
					ctx,
					promQuery.query,
					time.Time{},
					//promQuery.queryRange,
				)
				if err != nil {
					return errors.Wrap(err, "error querying Prometheus")
				}
				if len(warnings) > 0 {
					fmt.Printf("Warnings: %v\n", warnings)
				}
				if result == nil {
					fmt.Println("No results found")
					continue
				}

				for _, res := range result.(model.Vector) {
					containerId := sha1.Sum([]byte(res.Metric["namespace"] + "/" + res.Metric["pod"] + "/" + res.Metric["container"]))

					name := ""

					if promQuery.nameLabel != "" {
						name = string(res.Metric[promQuery.nameLabel])
					}

					newContainerMetric := &schema.PrometheusContainerMetric{
						ContainerId: containerId[:],
						Timestamp:   (res.Timestamp.UnixNano() - res.Timestamp.UnixNano()%(60*1000000000)) / 1000000,
						Group:       promQuery.metricGroup,
						Name:        name,
						Value:       float64(res.Value),
					}

					select {
					case upsertContainerMetrics <- newContainerMetric:
					case <-ctx.Done():
						return ctx.Err()
					}
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Second * 55):
				}
			}
		})
	}

	g.Go(func() error {
		return database.NewUpsert(pms.db, database.WithStatement(pms.promMetricClusterUpsertStmt(), 3)).Stream(ctx, upsertClusterMetrics)
	})

	g.Go(func() error {
		return database.NewUpsert(pms.db, database.WithStatement(pms.promMetricNodeUpsertStmt(), 4)).Stream(ctx, upsertNodeMetrics)
	})

	g.Go(func() error {
		return database.NewUpsert(pms.db, database.WithStatement(pms.promMetricPodUpsertStmt(), 4)).Stream(ctx, upsertPodMetrics)
	})

	g.Go(func() error {
		return database.NewUpsert(pms.db, database.WithStatement(pms.promMetricContainerUpsertStmt(), 4)).Stream(ctx, upsertContainerMetrics)
	})

	return g.Wait()
}

// Clean deletes metrics from the database if the belonging pod is deleted
//func (ms *MetricSync) Clean(ctx context.Context, deleteChannel <-chan contracts.KDelete) error {
//
//	g, ctx := errgroup.WithContext(ctx)
//
//	deletesPod := make(chan any)
//	deletesContainer := make(chan any)
//
//	g.Go(func() error {
//		defer close(deletesPod)
//		defer close(deletesContainer)
//
//		for {
//			select {
//			case kdelete, more := <-deleteChannel:
//				if !more {
//					return nil
//				}
//
//				deletesPod <- kdelete.ID()
//				deletesContainer <- kdelete.ID()
//
//			case <-ctx.Done():
//				return ctx.Err()
//			}
//		}
//	})
//
//	g.Go(func() error {
//		return database.NewDelete(ms.db, database.ByColumn("reference_id")).Stream(ctx, &schema.PodMetric{}, deletesPod)
//	})
//
//	g.Go(func() error {
//		return database.NewDelete(ms.db, database.ByColumn("pod_reference_id")).Stream(ctx, &schema.ContainerMetric{}, deletesContainer)
//	})
//
//	return g.Wait()
//}
//
// Clean deletes metrics from the database if the belonging node is deleted
//func (nms *NodeMetricSync) Clean(ctx context.Context, deleteChannel <-chan contracts.KDelete) error {
//
//	g, ctx := errgroup.WithContext(ctx)
//
//	deletes := make(chan any)
//
//	g.Go(func() error {
//		defer close(deletes)
//
//		for {
//			select {
//			case kdelete, more := <-deleteChannel:
//				if !more {
//					return nil
//				}
//
//				deletes <- kdelete.ID()
//
//			case <-ctx.Done():
//				return ctx.Err()
//			}
//		}
//	})
//
//	g.Go(func() error {
//		return database.NewDelete(nms.db, database.ByColumn("node_id")).Stream(ctx, &schema.NodeMetric{}, deletes)
//	})
//
//	return g.Wait()
//}
