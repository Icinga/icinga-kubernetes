package metrics

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/periodic"
	"github.com/icinga/icinga-go-library/types"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/pkg/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"golang.org/x/sync/errgroup"
	kcorev1 "k8s.io/api/core/v1"
	kcache "k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
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

// PromMetricSync synchronizes prometheus metrics from the prometheus API to the database
type PromMetricSync struct {
	promApiClient v1.API
	db            *database.DB
}

// NewPromMetricSync creates a new PromMetricSync
func NewPromMetricSync(promApiClient v1.API, db *database.DB) *PromMetricSync {
	return &PromMetricSync{
		promApiClient: promApiClient,
		db:            db,
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

	upsertNodeMetrics := make(chan database.Entity)

	g, ctx := errgroup.WithContext(ctx)

	for _, promQuery := range promQueriesNode {
		g.Go(func() error {
			for {
				result, warnings, err := pms.promApiClient.Query(
					ctx,
					promQuery.query,
					time.Time{},
				)
				if err != nil {
					return errors.Wrap(err, "error querying Prometheus")
				}
				if len(warnings) > 0 {
					klog.Warningf("Prometheus warnings: %v\n", warnings)
				}
				if result == nil {
					continue
				}

				for _, res := range result.(model.Vector) {
					if res.Value.String() == "NaN" {
						continue
					}

					nodeName := string(res.Metric["node"])
					if nodeName == "" {
						if strings.Contains(string(res.Metric["instance"]), ":") {
							if host, _, err := net.SplitHostPort(string(res.Metric["instance"])); err == nil {
								nodeName = host
							} else {
								continue
							}
						} else {
							nodeName = string(res.Metric["instance"])
						}
					}
					uuid, exists := nodes.Load(nodeName)
					if !exists {
						continue
					}

					name := ""
					if promQuery.nameLabel != "" {
						name = string(res.Metric[promQuery.nameLabel])
					}

					newNodeMetric := &schemav1.PrometheusNodeMetric{
						NodeUuid:  uuid.(types.UUID),
						Timestamp: (res.Timestamp.UnixNano() - res.Timestamp.UnixNano()%(60*1000000000)) / 1000000,
						Category:  promQuery.metricCategory,
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

	g.Go(func() error {
		return database.NewUpsert(pms.db, database.WithStatement(pms.promMetricNodeUpsertStmt(), 5)).Stream(ctx, upsertNodeMetrics)
	})

	return g.Wait()
}

func (pms *PromMetricSync) Pods(ctx context.Context, informer kcache.SharedIndexInformer) error {
	if !kcache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return errors.New("timed out waiting for caches to sync")
	}

	promQueriesPod := []PromQuery{
		{
			"cpu.usage",
			`sum by (node, namespace, pod) (rate(container_cpu_usage_seconds_total[1m]))`,
			"",
		},
		{
			"memory.usage",
			`sum by (node, namespace, pod) (container_memory_usage_bytes) / on (node) group_left(instance) label_replace(node_memory_MemTotal_bytes, "node", "$1", "instance", "(.*)")`,
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

	upsertPodMetrics := make(chan database.Entity)

	g, ctx := errgroup.WithContext(ctx)

	for _, promQuery := range promQueriesPod {
		g.Go(func() error {
			for {
				result, warnings, err := pms.promApiClient.Query(
					ctx,
					promQuery.query,
					time.Time{},
				)
				if err != nil {
					return errors.Wrap(err, "error querying Prometheus")
				}
				if len(warnings) > 0 {
					klog.Warningf("Prometheus warnings: %v\n", warnings)
				}
				if result == nil {
					continue
				}

				for _, res := range result.(model.Vector) {
					if res.Metric["pod"] == "" {
						continue
					}

					item, exists, err := informer.GetStore().GetByKey(
						kcache.NewObjectName(string(res.Metric["namespace"]), string(res.Metric["pod"])).String())
					if err != nil {
						return errors.Wrap(err, "can't get pod from store")
					}
					if !exists {
						continue
					}
					pod := item.(*kcorev1.Pod)

					name := ""
					if promQuery.nameLabel != "" {
						name = string(res.Metric[promQuery.nameLabel])
					}

					newPodMetric := &schemav1.PrometheusPodMetric{
						PodUuid:   schemav1.EnsureUUID(pod.UID),
						Timestamp: (res.Timestamp.UnixNano() - res.Timestamp.UnixNano()%(60*1000000000)) / 1000000,
						Category:  promQuery.metricCategory,
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
				case <-time.After(time.Second * 60):
				}
			}
		})
	}

	g.Go(func() error {
		return database.NewUpsert(pms.db, database.WithStatement(pms.promMetricPodUpsertStmt(), 5)).Stream(ctx, upsertPodMetrics)
	})

	return g.Wait()
}

// Run starts syncing the prometheus metrics to the database.
// Therefore, it gets a list of the metric queries.
func (pms *PromMetricSync) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	upsertClusterMetrics := make(chan database.Entity)
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

	for _, promQuery := range promQueriesCluster {
		promQuery := promQuery

		g.Go(func() error {
			for {
				result, warnings, err := pms.promApiClient.Query(
					ctx,
					promQuery.query,
					time.Time{},
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
					if res.Value.String() == "NaN" {
						continue
					}

					//clusterId := sha1.Sum([]byte(""))

					name := ""

					if promQuery.nameLabel != "" {
						name = string(res.Metric[promQuery.nameLabel])
					}

					newClusterMetric := &schemav1.PrometheusClusterMetric{
						Timestamp: (res.Timestamp.UnixNano() - res.Timestamp.UnixNano()%(60*1000000000)) / 1000000,
						Category:  promQuery.metricCategory,
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

	for _, promQuery := range promQueriesContainer {
		promQuery := promQuery

		g.Go(func() error {
			for {
				result, warnings, err := pms.promApiClient.Query(
					ctx,
					promQuery.query,
					time.Time{},
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
					if res.Value.String() == "NaN" {
						continue
					}

					//containerId := sha1.Sum([]byte(res.Metric["namespace"] + "/" + res.Metric["pod"] + "/" + res.Metric["container"]))

					name := ""

					if promQuery.nameLabel != "" {
						name = string(res.Metric[promQuery.nameLabel])
					}

					newContainerMetric := &schemav1.PrometheusContainerMetric{
						Timestamp: (res.Timestamp.UnixNano() - res.Timestamp.UnixNano()%(60*1000000000)) / 1000000,
						Category:  promQuery.metricCategory,
						Name:      name,
						Value:     float64(res.Value),
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
		return database.NewUpsert(pms.db, database.WithStatement(pms.promMetricClusterUpsertStmt(), 5)).Stream(ctx, upsertClusterMetrics)
	})

	g.Go(func() error {
		return database.NewUpsert(pms.db, database.WithStatement(pms.promMetricContainerUpsertStmt(), 5)).Stream(ctx, upsertContainerMetrics)
	})

	return g.Wait()
}
