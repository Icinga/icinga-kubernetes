package main

import (
	"context"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/periodic"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal"
	cachev1 "github.com/icinga/icinga-kubernetes/internal/cache/v1"
	"github.com/icinga/icinga-kubernetes/pkg/cluster"
	"github.com/icinga/icinga-kubernetes/pkg/com"
	"github.com/icinga/icinga-kubernetes/pkg/daemon"
	kdatabase "github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/metrics"
	"github.com/icinga/icinga-kubernetes/pkg/notifications"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	syncv1 "github.com/icinga/icinga-kubernetes/pkg/sync/v1"
	k8sMysql "github.com/icinga/icinga-kubernetes/schema/mysql"
	"github.com/okzk/sdnotify"
	"github.com/pkg/errors"
	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	v2 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"os"
	"strings"
	"sync"
	"time"
)

const expectedSchemaVersion = "0.2.0"

func main() {
	runtime.ReallyCrash = true

	var glue daemon.ConfigFlagGlue
	var showVersion bool
	var clusterName string

	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	pflag.BoolVar(&showVersion, "version", false, "print version and exit")
	pflag.StringVar(
		&glue.Config,
		"config",
		"",
		fmt.Sprintf("path to the config file (default: %s)", daemon.DefaultConfigPath),
	)
	pflag.StringVar(&clusterName, "cluster-name", "", "name of the current cluster")

	loadingRules := kclientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &kclientcmd.DefaultClientConfig
	pflag.StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to a kube config. Only required if out-of-cluster")

	overrides := kclientcmd.ConfigOverrides{}
	kflags := kclientcmd.RecommendedConfigOverrideFlags("")
	kflags.ContextOverrideFlags.Namespace = kclientcmd.FlagInfo{}
	kclientcmd.BindOverrideFlags(&overrides, pflag.CommandLine, kflags)

	pflag.Parse()

	if showVersion {
		internal.Version.Print("Icinga Kubernetes")
		os.Exit(0)
	}

	klog.Infof("Starting Icinga for Kubernetes (%s)", internal.Version.Version)

	kconfig, err := kclientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &overrides).ClientConfig()
	if err != nil {
		if kclientcmd.IsEmptyConfig(err) {
			klog.Fatal(
				"no configuration provided: set KUBECONFIG environment variable or --kubeconfig CLI flag to" +
					" a kubeconfig file with cluster access configured")
		}

		klog.Fatal(errors.Wrap(err, "cannot configure Kubernetes client"))
	}

	clientset, err := kubernetes.NewForConfig(kconfig)
	if err != nil {
		klog.Fatal(err)
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)
	log := klog.NewKlogr()

	var cfg daemon.Config

	if err = config.Load(&cfg, config.LoadOptions{
		Flags:      glue,
		EnvOptions: config.EnvOptions{Prefix: "ICINGA_FOR_KUBERNETES_"},
	}); err != nil {
		klog.Fatal(errors.Wrap(err, "can't create configuration"))
	}

	dbLog := log.WithName("database")
	kdb, err := kdatabase.NewFromConfig(&cfg.Database, dbLog)
	if err != nil {
		klog.Fatal(err)
	}

	// When started by systemd, NOTIFY_SOCKET is set by systemd for Type=notify supervised services, which was the
	// default setting for the Icinga for Kubernetes service. Before switching to Type=simple. For Type=notify,
	// we need to tell systemd, that Icinga for Kubernetes finished starting up.
	_ = sdnotify.Ready()

	if !kdb.Connect() {
		return
	}

	hasSchema, err := dbHasSchema(kdb, cfg.Database.Database)
	if err != nil {
		klog.Fatal(err)
	}

	g, ctx := errgroup.WithContext(context.Background())

	if hasSchema {
		var version string

		err = retry.WithBackoff(
			ctx,
			func(ctx context.Context) (err error) {
				query := "SELECT version FROM kubernetes_schema ORDER BY id DESC LIMIT 1"
				err = kdb.QueryRowxContext(ctx, query).Scan(&version)
				if err != nil {
					err = kdatabase.CantPerformQuery(err, query)
				}
				return
			},
			retry.Retryable,
			backoff.NewExponentialWithJitter(128*time.Millisecond, 1*time.Minute),
			retry.Settings{})
		if err != nil {
			klog.Fatal(err)
		}

		if version != expectedSchemaVersion {
			err = retry.WithBackoff(
				ctx,
				func(ctx context.Context) (err error) {
					rows, err := kdb.Query(
						kdb.Rebind("SELECT table_name FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=?"),
						cfg.Database.Database,
					)
					if err != nil {
						klog.Fatal(err)
					}
					defer func() {
						_ = rows.Close()
					}()

					dbLog.Info("Dropping schema")

					for rows.Next() {
						var tableName string
						if err := rows.Scan(&tableName); err != nil {
							klog.Fatal(err)
						}

						_, err := kdb.Exec(fmt.Sprintf(`DROP TABLE %s`, tableName))
						if err != nil {
							klog.Fatal(err)
						}
					}
					return
				},
				retry.Retryable,
				backoff.NewExponentialWithJitter(128*time.Millisecond, 1*time.Minute),
				retry.Settings{})
			if err != nil {
				klog.Fatal(err)
			}

			hasSchema = false
		}
	}

	if !hasSchema {
		dbLog.Info("Importing schema")

		for _, ddl := range strings.Split(k8sMysql.Schema, ";") {
			if ddl = strings.TrimSpace(ddl); ddl != "" {
				if _, err := kdb.Exec(ddl); err != nil {
					klog.Fatal(err)
				}
			}
		}
	}

	logs, err := logging.NewLoggingFromConfig("Icinga Kubernetes", cfg.Logging)
	if err != nil {
		klog.Fatal(errors.Wrap(err, "cannot configure logging"))
	}

	db, err := database.NewDbFromConfig(&cfg.Database, logs.GetChildLogger("database"), database.RetryConnectorCallbacks{})
	if err != nil {
		klog.Fatal("IGL_DATABASE: ", err)
	}

	namespaceName := "kube-system"
	ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespaceName, v1.GetOptions{})
	if err != nil {
		klog.Fatalf("Failed to retrieve namespace '%s' for cluster '%s': %v", namespaceName, clusterName, err)
	}

	clusterInstance := &schemav1.Cluster{
		Uuid: schemav1.EnsureUUID(ns.UID),
		Name: schemav1.NewNullableString(clusterName),
	}

	ctx = cluster.NewClusterUuidContext(ctx, clusterInstance.Uuid)

	stmt, _ := kdb.BuildUpsertStmt(clusterInstance)
	if _, err := kdb.NamedExecContext(ctx, stmt, clusterInstance); err != nil {
		klog.Error(errors.Wrap(err, "cannot update cluster"))
	}

	if _, err := kdb.ExecContext(ctx, "DELETE FROM kubernetes_instance WHERE cluster_uuid = ?", clusterInstance.Uuid); err != nil {
		klog.Fatal(errors.Wrap(err, "cannot delete instance"))
	}
	// ,omitempty
	var kubernetesVersion string
	var kubernetesHeartbeat time.Time
	instanceId := uuid.New()
	defer periodic.Start(ctx, 55*time.Second, func(tick periodic.Tick) {
		version, err := clientset.Discovery().ServerVersion()
		if err == nil {
			kubernetesVersion = version.GitVersion
			kubernetesHeartbeat = tick.Time
		}

		instance := schemav1.Instance{
			Uuid:                instanceId[:],
			ClusterUuid:         clusterInstance.Uuid,
			Version:             internal.Version.Version,
			KubernetesVersion:   schemav1.NewNullableString(kubernetesVersion),
			KubernetesHeartbeat: types.UnixMilli(kubernetesHeartbeat),
			KubernetesApiReachable: types.Bool{
				Bool:  err == nil,
				Valid: true,
			},
			Message:   schemav1.NewNullableString(err),
			Heartbeat: types.UnixMilli(tick.Time),
		}

		stmt, _ := kdb.BuildUpsertStmt(instance)

		if _, err := kdb.NamedExecContext(ctx, stmt, instance); err != nil {
			klog.Error(errors.Wrap(err, "cannot update instance"))
		}
	}, periodic.Immediate()).Stop()

	if err := internal.SyncNotificationsConfig(ctx, db, &cfg.Notifications, clusterInstance.Uuid); err != nil {
		klog.Fatal(err)
	}

	if cfg.Notifications.Url != "" {
		klog.Infof("Sending notifications to %s", cfg.Notifications.Url)

		nclient, err := notifications.NewClient("icinga-kubernetes/"+internal.Version.Version, cfg.Notifications)
		if err != nil {
			klog.Fatal(err)
		}

		g.Go(func() error {
			return nclient.Stream(ctx, cachev1.Multiplexers().Nodes().UpsertEvents().Out())
		})

		g.Go(func() error {
			return nclient.Stream(ctx, cachev1.Multiplexers().DaemonSets().UpsertEvents().Out())
		})

		g.Go(func() error {
			return nclient.Stream(ctx, cachev1.Multiplexers().StatefulSets().UpsertEvents().Out())
		})

		g.Go(func() error {
			return nclient.Stream(ctx, cachev1.Multiplexers().Deployments().UpsertEvents().Out())
		})

		g.Go(func() error {
			return nclient.Stream(ctx, cachev1.Multiplexers().ReplicaSets().UpsertEvents().Out())
		})

		g.Go(func() error {
			return nclient.Stream(ctx, cachev1.Multiplexers().Pods().UpsertEvents().Out())
		})
	}

	g.Go(func() error {
		return SyncServicePods(ctx, kdb, factory.Core().V1().Services(), factory.Core().V1().Pods())
	})

	err = internal.SyncPrometheusConfig(ctx, db, &cfg.Prometheus, clusterInstance.Uuid)
	if err != nil {
		klog.Error(errors.Wrap(err, "cannot sync prometheus config"))
	}

	if cfg.Prometheus.Url == "" {
		err = internal.AutoDetectPrometheus(ctx, clientset, &cfg.Prometheus)
		if err != nil {
			klog.Error(errors.Wrap(err, "cannot auto-detect prometheus"))
		}
	}

	if cfg.Prometheus.Url != "" {
		basicAuthTransport := &com.BasicAuthTransport{}

		if cfg.Prometheus.Insecure == "true" {
			basicAuthTransport.Insecure = true
		}

		if cfg.Prometheus.Username != "" {
			basicAuthTransport.Username = cfg.Prometheus.Username
			basicAuthTransport.Password = cfg.Prometheus.Password
		}

		promClient, err := promapi.NewClient(promapi.Config{
			Address:      cfg.Prometheus.Url,
			RoundTripper: basicAuthTransport,
		})
		if err != nil {
			klog.Fatal(errors.Wrap(err, "error creating Prometheus client"))
		}

		promApiClient := promv1.NewAPI(promClient)
		promMetricSync := metrics.NewPromMetricSync(promApiClient, db, logs.GetChildLogger("prometheus"))

		g.Go(func() error {
			return promMetricSync.Nodes(ctx, factory.Core().V1().Nodes().Informer())
		})

		g.Go(func() error {
			return promMetricSync.Pods(ctx, factory.Core().V1().Pods().Informer())
		})
	}

	g.Go(func() error {
		s := syncv1.NewSync(kdb, factory.Core().V1().Namespaces().Informer(), log.WithName("namespaces"), schemav1.NewNamespace)

		return s.Run(ctx)
	})

	wg := sync.WaitGroup{}

	wg.Add(1)
	g.Go(func() error {
		s := syncv1.NewSync(kdb, factory.Core().V1().Nodes().Informer(), log.WithName("nodes"), schemav1.NewNode)

		var forwardForNotifications []syncv1.Feature
		if cfg.Notifications.Url != "" {
			forwardForNotifications = append(
				forwardForNotifications,
				syncv1.WithOnUpsert(database.OnSuccessSendTo(cachev1.Multiplexers().Nodes().UpsertEvents().In())),
				syncv1.WithOnDelete(database.OnSuccessSendTo(cachev1.Multiplexers().Nodes().DeleteEvents().In())),
			)
		}

		wg.Done()

		return s.Run(ctx, forwardForNotifications...)
	})

	wg.Add(1)
	g.Go(func() error {
		schemav1.SyncContainers(
			ctx,
			kdb,
			g,
			cachev1.Multiplexers().Pods().UpsertEvents().Out(),
			cachev1.Multiplexers().Pods().DeleteEvents().Out(),
		)

		f := schemav1.NewPodFactory(clientset)
		s := syncv1.NewSync(kdb, factory.Core().V1().Pods().Informer(), log.WithName("pods"), f.New)

		wg.Done()

		return s.Run(
			ctx,
			syncv1.WithOnUpsert(database.OnSuccessSendTo(cachev1.Multiplexers().Pods().UpsertEvents().In())),
			syncv1.WithOnDelete(database.OnSuccessSendTo(cachev1.Multiplexers().Pods().DeleteEvents().In())),
		)
	})

	wg.Add(1)
	g.Go(func() error {
		s := syncv1.NewSync(
			kdb, factory.Apps().V1().Deployments().Informer(), log.WithName("deployments"), schemav1.NewDeployment)

		var forwardForNotifications []syncv1.Feature
		if cfg.Notifications.Url != "" {
			forwardForNotifications = append(
				forwardForNotifications,
				syncv1.WithOnUpsert(database.OnSuccessSendTo(cachev1.Multiplexers().Deployments().UpsertEvents().In())),
				syncv1.WithOnDelete(database.OnSuccessSendTo(cachev1.Multiplexers().Deployments().DeleteEvents().In())),
			)
		}

		wg.Done()

		return s.Run(ctx, forwardForNotifications...)
	})

	wg.Add(1)
	g.Go(func() error {
		s := syncv1.NewSync(
			kdb, factory.Apps().V1().DaemonSets().Informer(), log.WithName("daemon-sets"), schemav1.NewDaemonSet)

		var forwardForNotifications []syncv1.Feature
		if cfg.Notifications.Url != "" {
			forwardForNotifications = append(
				forwardForNotifications,
				syncv1.WithOnUpsert(database.OnSuccessSendTo(cachev1.Multiplexers().DaemonSets().UpsertEvents().In())),
				syncv1.WithOnDelete(database.OnSuccessSendTo(cachev1.Multiplexers().DaemonSets().DeleteEvents().In())),
			)
		}

		wg.Done()

		return s.Run(ctx, forwardForNotifications...)
	})

	wg.Add(1)
	g.Go(func() error {
		s := syncv1.NewSync(
			kdb, factory.Apps().V1().ReplicaSets().Informer(), log.WithName("replica-sets"), schemav1.NewReplicaSet)

		var forwardForNotifications []syncv1.Feature
		if cfg.Notifications.Url != "" {
			forwardForNotifications = append(
				forwardForNotifications,
				syncv1.WithOnUpsert(database.OnSuccessSendTo(cachev1.Multiplexers().ReplicaSets().UpsertEvents().In())),
				syncv1.WithOnDelete(database.OnSuccessSendTo(cachev1.Multiplexers().ReplicaSets().DeleteEvents().In())),
			)
		}

		wg.Done()

		return s.Run(ctx, forwardForNotifications...)
	})

	wg.Add(1)
	g.Go(func() error {
		s := syncv1.NewSync(
			kdb, factory.Apps().V1().StatefulSets().Informer(), log.WithName("stateful-sets"), schemav1.NewStatefulSet)

		var forwardForNotifications []syncv1.Feature
		if cfg.Notifications.Url != "" {
			forwardForNotifications = append(
				forwardForNotifications,
				syncv1.WithOnUpsert(database.OnSuccessSendTo(cachev1.Multiplexers().StatefulSets().UpsertEvents().In())),
				syncv1.WithOnDelete(database.OnSuccessSendTo(cachev1.Multiplexers().StatefulSets().DeleteEvents().In())),
			)
		}

		wg.Done()

		return s.Run(ctx, forwardForNotifications...)
	})

	g.Go(func() error {
		f := schemav1.NewServiceFactory(clientset)
		s := syncv1.NewSync(kdb, factory.Core().V1().Services().Informer(), log.WithName("services"), f.NewService)

		return s.Run(
			ctx,
			syncv1.WithOnUpsert(database.OnSuccessSendTo(cachev1.Multiplexers().Services().UpsertEvents().In())),
		)
	})

	g.Go(func() error {
		s := syncv1.NewSync(kdb, factory.Discovery().V1().EndpointSlices().Informer(), log.WithName("endpoints"), schemav1.NewEndpointSlice)

		return s.Run(ctx)
	})

	g.Go(func() error {
		s := syncv1.NewSync(kdb, factory.Core().V1().Secrets().Informer(), log.WithName("secrets"), schemav1.NewSecret)
		return s.Run(ctx)
	})

	g.Go(func() error {
		s := syncv1.NewSync(kdb, factory.Core().V1().ConfigMaps().Informer(), log.WithName("config-maps"), schemav1.NewConfigMap)

		return s.Run(ctx)
	})

	g.Go(func() error {
		s := syncv1.NewSync(kdb, factory.Events().V1().Events().Informer(), log.WithName("events"), schemav1.NewEvent)

		return s.Run(ctx, syncv1.WithNoDelete(), syncv1.WithNoWarumup())
	})

	g.Go(func() error {
		s := syncv1.NewSync(kdb, factory.Core().V1().PersistentVolumeClaims().Informer(), log.WithName("pvcs"), schemav1.NewPvc)

		return s.Run(ctx)
	})

	g.Go(func() error {
		s := syncv1.NewSync(kdb, factory.Core().V1().PersistentVolumes().Informer(), log.WithName("persistent-volumes"), schemav1.NewPersistentVolume)

		return s.Run(ctx)
	})

	g.Go(func() error {
		s := syncv1.NewSync(kdb, factory.Batch().V1().Jobs().Informer(), log.WithName("jobs"), schemav1.NewJob)

		return s.Run(ctx)
	})

	g.Go(func() error {
		s := syncv1.NewSync(kdb, factory.Batch().V1().CronJobs().Informer(), log.WithName("cron-jobs"), schemav1.NewCronJob)

		return s.Run(ctx)
	})

	g.Go(func() error {
		s := syncv1.NewSync(kdb, factory.Networking().V1().Ingresses().Informer(), log.WithName("ingresses"), schemav1.NewIngress)

		return s.Run(ctx)
	})

	g.Go(func() error {
		wg.Wait()

		klog.V(2).Info("Starting multiplexers")

		return cachev1.Multiplexers().Run(ctx)
	})

	g.Go(func() error {
		return kdb.PeriodicCleanup(ctx, kdatabase.CleanupStmt{
			Table:  "event",
			PK:     "uuid",
			Column: "created",
		})
	})

	g.Go(func() error {
		return kdb.PeriodicCleanup(ctx, kdatabase.CleanupStmt{
			Table:  "prometheus_cluster_metric",
			PK:     "(cluster_uuid, timestamp, category, name)",
			Column: "timestamp",
		})
	})

	g.Go(func() error {
		return kdb.PeriodicCleanup(ctx, kdatabase.CleanupStmt{
			Table:  "prometheus_node_metric",
			PK:     "(node_uuid, timestamp, category, name)",
			Column: "timestamp",
		})
	})

	g.Go(func() error {
		return kdb.PeriodicCleanup(ctx, kdatabase.CleanupStmt{
			Table:  "prometheus_pod_metric",
			PK:     "(pod_uuid, timestamp, category, name)",
			Column: "timestamp",
		})
	})

	g.Go(func() error {
		return kdb.PeriodicCleanup(ctx, kdatabase.CleanupStmt{
			Table:  "prometheus_container_metric",
			PK:     "(container_uuid, timestamp, category, name)",
			Column: "timestamp",
		})
	})

	if err := g.Wait(); err != nil {
		klog.Fatal(err)
	}
}

// dbHasSchema queries via db whether the database dbName has a table named "kubernetes_schema".
func dbHasSchema(db *kdatabase.Database, dbName string) (bool, error) {
	rows, err := db.Query(
		db.Rebind("SELECT 1 FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME='kubernetes_schema'"),
		dbName,
	)
	if err != nil {
		return false, err
	}

	defer func() { _ = rows.Close() }()

	return rows.Next(), rows.Err()
}

func SyncServicePods(ctx context.Context, db *kdatabase.Database, serviceList v2.ServiceInformer, podList v2.PodInformer) error {
	servicePods := make(chan any)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return db.UpsertStreamed(ctx, servicePods)
	})

	g.Go(func() error {
		ch := cachev1.Multiplexers().Pods().UpsertEvents().Out()
		for {
			select {
			case pod, more := <-ch:
				if !more {
					return nil
				}

				services, err := serviceList.Lister().List(labels.NewSelector())
				if err != nil {
					return err
				}

				podLabels := make(labels.Set)
				for _, label := range pod.(*schemav1.Pod).Labels {
					podLabels[label.Name] = label.Value
				}

				for _, service := range services {
					if len(service.Spec.Selector) == 0 {
						continue
					}

					labelSelector := &v1.LabelSelector{MatchLabels: service.Spec.Selector}
					selector, err := v1.LabelSelectorAsSelector(labelSelector)
					if err != nil {
						return err
					}

					if selector.Matches(podLabels) {
						select {
						case servicePods <- schemav1.ServicePod{
							ServiceUuid: schemav1.EnsureUUID(service.UID),
							PodUuid:     pod.(*schemav1.Pod).Uuid,
						}:
						case <-ctx.Done():
							return ctx.Err()
						}
					}
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	g.Go(func() error {
		ch := cachev1.Multiplexers().Services().UpsertEvents().Out()
		for {
			select {
			case service, more := <-ch:
				if !more {
					return nil
				}

				if len(service.(*schemav1.Service).Selectors) == 0 {
					continue
				}

				labelSelector := &v1.LabelSelector{MatchLabels: map[string]string{}}
				for _, selector := range service.(*schemav1.Service).Selectors {
					labelSelector.MatchLabels[selector.Name] = selector.Value
				}

				selector, err := v1.LabelSelectorAsSelector(labelSelector)
				if err != nil {
					return err
				}

				pods, err := podList.Lister().List(selector)
				if err != nil {
					return err
				}

				for _, pod := range pods {
					select {
					case servicePods <- schemav1.ServicePod{
						ServiceUuid: service.(*schemav1.Service).Uuid,
						PodUuid:     schemav1.EnsureUUID(pod.UID),
					}:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	g.Go(func() error {
		ch := cachev1.Multiplexers().Pods().DeleteEvents().Out()
		for {
			select {
			case podUuid, more := <-ch:
				if !more {
					return nil
				}

				_, err := db.ExecContext(ctx, `DELETE FROM service_pod WHERE pod_uuid = ?`, podUuid)
				if err != nil {
					return err
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return g.Wait()
}
