package main

import (
	"context"
	"flag"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/icinga/icinga-go-library/config"
	igldatabase "github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/periodic"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal"
	"github.com/icinga/icinga-kubernetes/pkg/backoff"
	"github.com/icinga/icinga-kubernetes/pkg/com"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/metrics"
	"github.com/icinga/icinga-kubernetes/pkg/retry"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/icinga/icinga-kubernetes/pkg/sync"
	syncv1 "github.com/icinga/icinga-kubernetes/pkg/sync/v1"
	k8sMysql "github.com/icinga/icinga-kubernetes/schema/mysql"
	"github.com/okzk/sdnotify"
	"github.com/pkg/errors"
	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"os"
	"strings"
	"time"
)

const expectedSchemaVersion = "0.2.0"

func main() {
	runtime.ReallyCrash = true

	var configLocation string
	var showVersion bool

	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	pflag.BoolVar(&showVersion, "version", false, "print version and exit")
	pflag.StringVar(&configLocation, "config", "./config.yml", "path to the config file")

	loadingRules := kclientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &kclientcmd.DefaultClientConfig
	pflag.StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to a kube config. Only required if out-of-cluster")

	overrides := kclientcmd.ConfigOverrides{}
	kflags := kclientcmd.RecommendedConfigOverrideFlags("")
	kflags.ContextOverrideFlags.Namespace = kclientcmd.FlagInfo{}
	kclientcmd.BindOverrideFlags(&overrides, pflag.CommandLine, kflags)

	pflag.Parse()

	if showVersion {
		internal.Version.Print()
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

		klog.Fatal(errors.Wrap(err, "can't configure Kubernetes client"))
	}

	clientset, err := kubernetes.NewForConfig(kconfig)
	if err != nil {
		klog.Fatal(err)
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)
	log := klog.NewKlogr()

	var cfg internal.Config
	err = config.FromYAMLFile(configLocation, &cfg)
	if err != nil {
		klog.Fatal(errors.Wrap(err, "can't create configuration"))
	}

	dbLog := log.WithName("database")
	db, err := database.NewFromConfig(&cfg.Database, dbLog)
	if err != nil {
		klog.Fatal(err)
	}

	// When started by systemd, NOTIFY_SOCKET is set by systemd for Type=notify supervised services, which was the
	// default setting for the Icinga for Kubernetes service. Before switching to Type=simple. For Type=notify,
	// we need to tell systemd, that Icinga for Kubernetes finished starting up.
	_ = sdnotify.Ready()

	if !db.Connect() {
		return
	}

	hasSchema, err := dbHasSchema(db, cfg.Database.Database)
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
				err = db.QueryRowxContext(ctx, query).Scan(&version)
				if err != nil {
					err = database.CantPerformQuery(err, query)
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
					rows, err := db.Query(
						db.Rebind("SELECT table_name FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=?"),
						cfg.Database.Database,
					)
					if err != nil {
						klog.Fatal(err)
					}
					defer rows.Close()

					dbLog.Info("Dropping schema")

					for rows.Next() {
						var tableName string
						if err := rows.Scan(&tableName); err != nil {
							klog.Fatal(err)
						}

						_, err := db.Exec("DROP TABLE " + tableName)
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
				if _, err := db.Exec(ddl); err != nil {
					klog.Fatal(err)
				}
			}
		}
	}

	if _, err := db.ExecContext(ctx, "DELETE FROM kubernetes_instance"); err != nil {
		klog.Fatal(errors.Wrap(err, "can't delete instance"))
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

		stmt, _ := db.BuildUpsertStmt(instance)

		if _, err := db.NamedExecContext(ctx, stmt, instance); err != nil {
			klog.Error(errors.Wrap(err, "can't update instance"))
		}
	}, periodic.Immediate()).Stop()

	if cfg.Prometheus.Url != "" {
		logs, err := logging.NewLoggingFromConfig("Icinga Kubernetes", cfg.Logging)
		if err != nil {
			klog.Fatal(errors.Wrap(err, "can't configure logging"))
		}

		db2, err := igldatabase.NewDbFromConfig(&cfg.Database, logs.GetChildLogger("database"), igldatabase.RetryConnectorCallbacks{})
		if err != nil {
			klog.Fatal("IGL_DATABASE: ", err)
		}

		promClient, err := promapi.NewClient(promapi.Config{Address: cfg.Prometheus.Url})
		if err != nil {
			klog.Fatal(errors.Wrap(err, "error creating promClient"))
		}

		promApiClient := promv1.NewAPI(promClient)
		promMetricSync := metrics.NewPromMetricSync(promApiClient, db2, logs.GetChildLogger("prometheus"))

		g.Go(func() error {
			return promMetricSync.Nodes(ctx, factory.Core().V1().Nodes().Informer())
		})

		g.Go(func() error {
			return promMetricSync.Pods(ctx, factory.Core().V1().Pods().Informer())
		})
	}

	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Core().V1().Namespaces().Informer(), log.WithName("namespaces"), schemav1.NewNamespace)

		return s.Run(ctx)
	})
	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Core().V1().Nodes().Informer(), log.WithName("nodes"), schemav1.NewNode)

		return s.Run(ctx)
	})
	g.Go(func() error {
		pods := make(chan any)
		deletePodIds := make(chan interface{})
		defer close(pods)
		defer close(deletePodIds)

		schemav1.SyncContainers(ctx, db, g, pods, deletePodIds)

		f := schemav1.NewPodFactory(clientset)
		s := syncv1.NewSync(db, factory.Core().V1().Pods().Informer(), log.WithName("pods"), f.New)

		return s.Run(ctx, sync.WithOnUpsert(com.ForwardBulk(pods)), sync.WithOnDelete(com.ForwardBulk(deletePodIds)))
	})
	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Apps().V1().Deployments().Informer(), log.WithName("deployments"), schemav1.NewDeployment)

		return s.Run(ctx)
	})
	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Apps().V1().DaemonSets().Informer(), log.WithName("daemon-sets"), schemav1.NewDaemonSet)

		return s.Run(ctx)
	})
	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Apps().V1().ReplicaSets().Informer(), log.WithName("replica-sets"), schemav1.NewReplicaSet)

		return s.Run(ctx)
	})
	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Apps().V1().StatefulSets().Informer(), log.WithName("stateful-sets"), schemav1.NewStatefulSet)

		return s.Run(ctx)
	})
	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Core().V1().Services().Informer(), log.WithName("services"), schemav1.NewService)

		return s.Run(ctx)
	})
	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Discovery().V1().EndpointSlices().Informer(), log.WithName("endpoints"), schemav1.NewEndpointSlice)

		return s.Run(ctx)
	})
	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Core().V1().Secrets().Informer(), log.WithName("secrets"), schemav1.NewSecret)
		return s.Run(ctx)
	})
	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Core().V1().ConfigMaps().Informer(), log.WithName("config-maps"), schemav1.NewConfigMap)

		return s.Run(ctx)
	})
	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Events().V1().Events().Informer(), log.WithName("events"), schemav1.NewEvent)

		return s.Run(ctx, sync.WithNoDelete(), sync.WithNoWarumup())
	})
	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Core().V1().PersistentVolumeClaims().Informer(), log.WithName("pvcs"), schemav1.NewPvc)

		return s.Run(ctx)
	})
	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Core().V1().PersistentVolumes().Informer(), log.WithName("persistent-volumes"), schemav1.NewPersistentVolume)

		return s.Run(ctx)
	})
	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Batch().V1().Jobs().Informer(), log.WithName("jobs"), schemav1.NewJob)

		return s.Run(ctx)
	})
	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Batch().V1().CronJobs().Informer(), log.WithName("cron-jobs"), schemav1.NewCronJob)

		return s.Run(ctx)
	})
	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Networking().V1().Ingresses().Informer(), log.WithName("ingresses"), schemav1.NewIngress)

		return s.Run(ctx)
	})

	g.Go(func() error {
		return db.PeriodicCleanup(ctx, database.CleanupStmt{
			Table:  "event",
			PK:     "uuid",
			Column: "created",
		})
	})

	g.Go(func() error {
		return db.PeriodicCleanup(ctx, database.CleanupStmt{
			Table:  "prometheus_cluster_metric",
			PK:     "(cluster_uuid, timestamp, category, name)",
			Column: "timestamp",
		})
	})

	g.Go(func() error {
		return db.PeriodicCleanup(ctx, database.CleanupStmt{
			Table:  "prometheus_node_metric",
			PK:     "(node_uuid, timestamp, category, name)",
			Column: "timestamp",
		})
	})

	g.Go(func() error {
		return db.PeriodicCleanup(ctx, database.CleanupStmt{
			Table:  "prometheus_pod_metric",
			PK:     "(pod_uuid, timestamp, category, name)",
			Column: "timestamp",
		})
	})

	g.Go(func() error {
		return db.PeriodicCleanup(ctx, database.CleanupStmt{
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
func dbHasSchema(db *database.Database, dbName string) (bool, error) {
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
