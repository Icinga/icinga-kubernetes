package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	stdsync "sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/periodic"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/icinga/icinga-kubernetes/internal/cache"
	"github.com/icinga/icinga-kubernetes/internal/cluster"
	"github.com/icinga/icinga-kubernetes/internal/com"
	"github.com/icinga/icinga-kubernetes/internal/daemon"
	"github.com/icinga/icinga-kubernetes/internal/kdatabase"
	"github.com/icinga/icinga-kubernetes/internal/metrics"
	"github.com/icinga/icinga-kubernetes/internal/notifications"
	"github.com/icinga/icinga-kubernetes/internal/schema"
	"github.com/icinga/icinga-kubernetes/internal/sync"
	k8sMysql "github.com/icinga/icinga-kubernetes/schema/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/okzk/sdnotify"
	"github.com/pkg/errors"
	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	kcorev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	corev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	kcache "k8s.io/client-go/tools/cache"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

const expectedSchemaVersion = "0.5.0"

func main() {
	var glue daemon.ConfigFlagGlue
	var showVersion bool
	var clusterName string
	var removeClusterUuids uuidSliceValue

	// The Kubernetes client libraries log through klog, so keep its flags on our command line
	// to let -v and friends control their output. Our own code logs via icinga-go-library.
	klogFlags := flag.NewFlagSet("klog", flag.ContinueOnError)
	klog.InitFlags(klogFlags)
	pflag.CommandLine.AddGoFlagSet(klogFlags)

	pflag.BoolVar(&showVersion, "version", false, "print version and exit")
	pflag.StringVar(&glue.Config, "config", "", fmt.Sprintf("path to the config file (default: %s)", daemon.DefaultConfigPath))
	pflag.StringVar(&clusterName, "cluster-name", "", "name of the current cluster")
	pflag.Var(&removeClusterUuids, "remove-cluster", "remove clusters from the database by UUID (repeatable or comma-separated)")

	loadingRules := kclientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &kclientcmd.DefaultClientConfig
	pflag.StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster")

	overrides := &kclientcmd.ConfigOverrides{}
	kflags := kclientcmd.RecommendedConfigOverrideFlags("")
	kflags.ContextOverrideFlags.Namespace = kclientcmd.FlagInfo{}
	kclientcmd.BindOverrideFlags(overrides, pflag.CommandLine, kflags)

	pflag.Parse()

	if showVersion {
		daemon.Version.Print("Icinga for Kubernetes")
		os.Exit(0)
	}

	var cfg daemon.Config

	if err := config.Load(&cfg, config.LoadOptions{
		Flags:      glue,
		EnvOptions: config.EnvOptions{Prefix: "ICINGA_FOR_KUBERNETES_"},
	}); err != nil {
		utils.PrintErrorThenExit(errors.Wrap(err, "cannot load configuration"), 1)
	}

	logs, err := logging.NewLoggingFromConfig("Icinga for Kubernetes", cfg.Logging.Config)
	if err != nil {
		utils.PrintErrorThenExit(errors.Wrap(err, "cannot configure logging"), 1)
	}

	logger := logs.GetLogger()

	if len(removeClusterUuids.Value()) == 0 {
		logger.Infow("Starting Icinga for Kubernetes", zap.String("version", daemon.Version.Version))
	}

	// An explicit -v/--v on the command line wins over the config file.
	if !pflag.CommandLine.Changed("v") {
		if err := pflag.CommandLine.Set("v", strconv.Itoa(int(cfg.Logging.Kubernetes))); err != nil {
			logger.Fatalw("Cannot set Kubernetes logging verbosity", logging.Error(err))
		}
	}

	dbLog := logs.GetChildLogger("database")

	db, err := kdatabase.NewFromConfig(&cfg.Database, dbLog)
	if err != nil {
		logger.Fatalw("Cannot create database", logging.Error(err))
	}
	defer func() { _ = db.Close() }()

	logger.Infow("Connecting to database", zap.String("address", db.GetAddr()))
	if err := db.Ping(); err != nil {
		logger.Fatalw("Cannot connect to database", logging.Error(err))
	}

	if len(removeClusterUuids.Value()) != 0 {
		clusterRemovalLogger := logs.GetChildLogger("cluster-removal")

		for _, u := range removeClusterUuids.Value() {
			clusterRemovalLogger.Infow("Removing cluster", zap.String("uuid", u.String()))
			if err := removeCluster(context.Background(), db, clusterRemovalLogger, u); err != nil {
				clusterRemovalLogger.Fatalw("Cannot remove cluster", zap.String("uuid", u.String()), logging.Error(err))
			}
		}

		return
	}

	kconfig, err := kclientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		if kclientcmd.IsEmptyConfig(err) {
			logger.Fatal(
				"No configuration provided: set KUBECONFIG environment variable or --kubeconfig CLI flag to" +
					" a kubeconfig file with cluster access configured")
		}

		logger.Fatalw("Cannot configure Kubernetes client", logging.Error(err))
	}

	if serverName, ok := os.LookupEnv("KUBERNETES_SERVER"); ok {
		kconfig.Host = serverName
	}

	clientset, err := kubernetes.NewForConfig(kconfig)
	if err != nil {
		logger.Fatalw("Cannot create Kubernetes clientset", logging.Error(err))
	}

	logger.Infow("Using Kubernetes API server", zap.String("host", kconfig.Host))

	hasSchema, err := dbHasSchema(db, cfg.Database.Database)
	if err != nil {
		logger.Fatalw("Cannot query whether the database has a kubernetes_schema table", logging.Error(err))
	}

	g, ctx := errgroup.WithContext(context.Background())

	if hasSchema {
		var version string

		if err := retry.WithBackoff(
			ctx,
			func(ctx context.Context) (err error) {
				query := `SELECT "version" FROM "kubernetes_schema" ORDER BY "id" DESC LIMIT 1`
				err = db.QueryRowxContext(ctx, query).Scan(&version)
				if err != nil {
					err = database.CantPerformQuery(err, query)
				}
				return
			},
			retry.Retryable,
			backoff.DefaultBackoff,
			db.GetDefaultRetrySettings(),
		); err != nil {
			logger.Fatalw("Cannot query schema version", logging.Error(err))
		}

		if version != expectedSchemaVersion {
			if err := retry.WithBackoff(
				ctx,
				func(ctx context.Context) error {
					query := db.Rebind(`SELECT "table_name" FROM "INFORMATION_SCHEMA"."TABLES" WHERE "TABLE_SCHEMA"=?`)
					rows, err := db.QueryContext(ctx, query, cfg.Database.Database)
					if err != nil {
						return database.CantPerformQuery(err, query)
					}
					defer func() {
						_ = rows.Close()
					}()

					dbLog.Info("Dropping schema")

					for rows.Next() {
						var tableName string
						if err := rows.Scan(&tableName); err != nil {
							return errors.Wrap(err, "cannot scan table name")
						}

						stmt := fmt.Sprintf(`DROP TABLE %q`, tableName)
						if _, err := db.ExecContext(ctx, stmt); err != nil {
							return database.CantPerformQuery(err, stmt)
						}
					}

					if err := rows.Err(); err != nil {
						return database.CantPerformQuery(err, query)
					}

					return nil
				},
				retry.Retryable,
				backoff.DefaultBackoff,
				db.GetDefaultRetrySettings(),
			); err != nil {
				logger.Fatalw("Cannot wipe outdated database tables", logging.Error(err))
			}

			hasSchema = false
		}
	}

	if !hasSchema {
		dbLog.Info("Importing schema")

		for ddl := range strings.SplitSeq(k8sMysql.Schema, ";") {
			if ddl = strings.TrimSpace(ddl); ddl != "" {
				if _, err := db.Exec(ddl); err != nil {
					logger.Fatalw("Cannot execute schema statement", logging.Error(err))
				}
			}
		}
	}

	if clusterName == "" {
		clusterName = os.Getenv("ICINGA_FOR_KUBERNETES_CLUSTER_NAME")
	}

	namespaceName := "kube-system"
	ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespaceName, v1.GetOptions{})
	if err != nil {
		logger.Fatalw(
			"Cannot retrieve namespace",
			zap.String("namespace", namespaceName),
			zap.String("cluster", clusterName),
			logging.Error(err),
		)
	}

	clusterInstance := &schema.Cluster{
		Uuid: schema.EnsureUUID(ns.UID),
		Name: schema.NewNullableString(clusterName),
	}

	ctx = cluster.NewClusterUuidContext(ctx, clusterInstance.Uuid)

	stmt, _ := db.BuildUpsertStmt(clusterInstance)
	if _, err := db.NamedExecContext(ctx, stmt, clusterInstance); err != nil {
		logger.Errorw("Cannot update cluster", logging.Error(err))
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM "kubernetes_instance" WHERE "cluster_uuid" = ?`, clusterInstance.Uuid); err != nil {
		logger.Fatalw("Cannot delete instance", logging.Error(err))
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

		instance := schema.Instance{
			Uuid:                types.MakeUUID(instanceId),
			ClusterUuid:         clusterInstance.Uuid,
			Version:             daemon.Version.Version,
			KubernetesVersion:   schema.NewNullableString(kubernetesVersion),
			KubernetesHeartbeat: types.UnixMilli(kubernetesHeartbeat),
			KubernetesApiReachable: types.Bool{
				Bool:  err == nil,
				Valid: true,
			},
			Message:   schema.NewNullableString(err),
			Heartbeat: types.UnixMilli(tick.Time),
		}

		stmt, _ := db.BuildUpsertStmt(instance)

		if _, err := db.NamedExecContext(ctx, stmt, instance); err != nil {
			logger.Errorw("Cannot update instance", logging.Error(err))
		}
	}, periodic.Immediate()).Stop()

	if err := daemon.SyncNotificationsConfig(ctx, db.DB, &cfg.Notifications, clusterInstance.Uuid); err != nil {
		logger.Fatalw("Cannot sync notifications configuration", logging.Error(err))
	}

	if cfg.Notifications.Url != "" {
		logger.Infow("Sending notifications", zap.String("url", cfg.Notifications.Url))

		nclient, err := notifications.NewClient(
			"icinga-kubernetes/"+daemon.Version.Version,
			cfg.Notifications,
			db.DB,
			logs.GetChildLogger("notifications"),
			clusterName,
		)
		if err != nil {
			logger.Fatalw("Cannot create notifications client", logging.Error(err))
		}

		defer periodic.Start(ctx, time.Hour, func(tick periodic.Tick) {
			nclient.CloseOrphanedIncidents(ctx, clusterInstance.Uuid)
		}, periodic.Immediate()).Stop()

		g.Go(func() error {
			return nclient.Stream(ctx, cache.Multiplexers().Nodes().UpsertEvents().Out())
		})

		g.Go(func() error {
			return nclient.Stream(ctx, cache.Multiplexers().DaemonSets().UpsertEvents().Out())
		})

		g.Go(func() error {
			return nclient.Stream(ctx, cache.Multiplexers().StatefulSets().UpsertEvents().Out())
		})

		g.Go(func() error {
			return nclient.Stream(ctx, cache.Multiplexers().Deployments().UpsertEvents().Out())
		})

		g.Go(func() error {
			return nclient.Stream(ctx, cache.Multiplexers().ReplicaSets().UpsertEvents().Out())
		})

		g.Go(func() error {
			return nclient.Stream(ctx, cache.Multiplexers().Pods().UpsertEvents().Out())
		})
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)

	wg := stdsync.WaitGroup{}

	wg.Add(1)
	g.Go(func() error {
		podEvents := cache.Multiplexers().Pods().UpsertEvents().Out()
		podDeletes := cache.Multiplexers().Pods().DeleteEvents().Out()
		serviceEvents := cache.Multiplexers().Services().UpsertEvents().Out()
		serviceDeletes := cache.Multiplexers().Services().DeleteEvents().Out()

		wg.Done()

		return SyncServicePods(
			ctx,
			db,
			factory.Core().V1().Services(),
			factory.Core().V1().Pods(),
			podEvents,
			podDeletes,
			serviceEvents,
			serviceDeletes,
		)
	})

	if err := daemon.SyncPrometheusConfig(ctx, db.DB, &cfg.Prometheus, clusterInstance.Uuid); err != nil {
		logger.Errorw("Cannot sync prometheus config", logging.Error(err))
	}

	if cfg.Prometheus.Url == "" {
		if err := daemon.AutoDetectPrometheus(ctx, clientset, &cfg.Prometheus); err != nil {
			logger.Errorw("Cannot auto-detect prometheus", logging.Error(err))
		}
	}

	if cfg.Prometheus.Url != "" {
		var transport http.RoundTripper = com.NewTransport(cfg.Prometheus.Insecure == "true")

		if cfg.Prometheus.Username != "" {
			transport = com.NewBasicAuthTransport(transport, cfg.Prometheus.Username, cfg.Prometheus.Password)
		}

		promClient, err := promapi.NewClient(promapi.Config{
			Address:      cfg.Prometheus.Url,
			RoundTripper: transport,
		})
		if err != nil {
			logger.Fatalw("Cannot create Prometheus client", logging.Error(err))
		}

		promApiClient := promv1.NewAPI(promClient)
		promMetricSync := metrics.NewPromMetricSync(promApiClient, db.DB, logs.GetChildLogger("prometheus"))

		g.Go(func() error {
			return promMetricSync.Nodes(ctx, factory.Core().V1().Nodes().Informer())
		})

		g.Go(func() error {
			return promMetricSync.Pods(ctx, factory.Core().V1().Pods().Informer())
		})
	}

	g.Go(func() error {
		s := sync.NewSync(db, factory.Core().V1().Namespaces().Informer(), logs.GetChildLogger("namespaces"), schema.NewNamespace)

		return s.Run(ctx)
	})

	wg.Add(1)
	g.Go(func() error {
		s := sync.NewSync(db, factory.Core().V1().Nodes().Informer(), logs.GetChildLogger("nodes"), schema.NewNode)

		var forwardForNotifications []sync.Feature
		if cfg.Notifications.Url != "" {
			forwardForNotifications = append(
				forwardForNotifications,
				sync.WithOnUpsert(database.OnSuccessSendTo(cache.Multiplexers().Nodes().UpsertEvents().In())),
				sync.WithOnDelete(database.OnSuccessSendTo(cache.Multiplexers().Nodes().DeleteEvents().In())),
			)
		}

		wg.Done()

		return s.Run(ctx, forwardForNotifications...)
	})

	wg.Add(1)
	g.Go(func() error {
		schema.SyncContainers(
			ctx,
			db,
			g,
			cache.Multiplexers().Pods().UpsertEvents().Out(),
			cache.Multiplexers().Pods().DeleteEvents().Out(),
		)

		f := schema.NewPodFactory(clientset)
		s := sync.NewSync(db, factory.Core().V1().Pods().Informer(), logs.GetChildLogger("pods"), f.New)

		wg.Done()

		return s.Run(
			ctx,
			sync.WithOnUpsert(database.OnSuccessSendTo(cache.Multiplexers().Pods().UpsertEvents().In())),
			sync.WithOnDelete(database.OnSuccessSendTo(cache.Multiplexers().Pods().DeleteEvents().In())),
		)
	})

	wg.Add(1)
	g.Go(func() error {
		s := sync.NewSync(db, factory.Apps().V1().Deployments().Informer(), logs.GetChildLogger("deployments"), schema.NewDeployment)

		var forwardForNotifications []sync.Feature
		if cfg.Notifications.Url != "" {
			forwardForNotifications = append(
				forwardForNotifications,
				sync.WithOnUpsert(database.OnSuccessSendTo(cache.Multiplexers().Deployments().UpsertEvents().In())),
				sync.WithOnDelete(database.OnSuccessSendTo(cache.Multiplexers().Deployments().DeleteEvents().In())),
			)
		}

		wg.Done()

		return s.Run(ctx, forwardForNotifications...)
	})

	wg.Add(1)
	g.Go(func() error {
		s := sync.NewSync(db, factory.Apps().V1().DaemonSets().Informer(), logs.GetChildLogger("daemon-sets"), schema.NewDaemonSet)

		var forwardForNotifications []sync.Feature
		if cfg.Notifications.Url != "" {
			forwardForNotifications = append(
				forwardForNotifications,
				sync.WithOnUpsert(database.OnSuccessSendTo(cache.Multiplexers().DaemonSets().UpsertEvents().In())),
				sync.WithOnDelete(database.OnSuccessSendTo(cache.Multiplexers().DaemonSets().DeleteEvents().In())),
			)
		}

		wg.Done()

		return s.Run(ctx, forwardForNotifications...)
	})

	wg.Add(1)
	g.Go(func() error {
		s := sync.NewSync(db, factory.Apps().V1().ReplicaSets().Informer(), logs.GetChildLogger("replica-sets"), schema.NewReplicaSet)

		var forwardForNotifications []sync.Feature
		if cfg.Notifications.Url != "" {
			forwardForNotifications = append(
				forwardForNotifications,
				sync.WithOnUpsert(database.OnSuccessSendTo(cache.Multiplexers().ReplicaSets().UpsertEvents().In())),
				sync.WithOnDelete(database.OnSuccessSendTo(cache.Multiplexers().ReplicaSets().DeleteEvents().In())),
			)
		}

		wg.Done()

		return s.Run(ctx, forwardForNotifications...)
	})

	wg.Add(1)
	g.Go(func() error {
		s := sync.NewSync(db, factory.Apps().V1().StatefulSets().Informer(), logs.GetChildLogger("stateful-sets"), schema.NewStatefulSet)

		var forwardForNotifications []sync.Feature
		if cfg.Notifications.Url != "" {
			forwardForNotifications = append(
				forwardForNotifications,
				sync.WithOnUpsert(database.OnSuccessSendTo(cache.Multiplexers().StatefulSets().UpsertEvents().In())),
				sync.WithOnDelete(database.OnSuccessSendTo(cache.Multiplexers().StatefulSets().DeleteEvents().In())),
			)
		}

		wg.Done()

		return s.Run(ctx, forwardForNotifications...)
	})

	g.Go(func() error {
		f := schema.NewServiceFactory(clientset)
		s := sync.NewSync(db, factory.Core().V1().Services().Informer(), logs.GetChildLogger("services"), f.NewService)

		return s.Run(
			ctx,
			sync.WithOnUpsert(database.OnSuccessSendTo(cache.Multiplexers().Services().UpsertEvents().In())),
			sync.WithOnDelete(database.OnSuccessSendTo(cache.Multiplexers().Services().DeleteEvents().In())),
		)
	})

	g.Go(func() error {
		s := sync.NewSync(db, factory.Discovery().V1().EndpointSlices().Informer(), logs.GetChildLogger("endpoints"), schema.NewEndpointSlice)

		return s.Run(ctx)
	})

	g.Go(func() error {
		s := sync.NewSync(db, factory.Core().V1().Secrets().Informer(), logs.GetChildLogger("secrets"), schema.NewSecret)
		return s.Run(ctx)
	})

	g.Go(func() error {
		s := sync.NewSync(db, factory.Core().V1().ConfigMaps().Informer(), logs.GetChildLogger("config-maps"), schema.NewConfigMap)

		return s.Run(ctx)
	})

	g.Go(func() error {
		s := sync.NewSync(db, factory.Events().V1().Events().Informer(), logs.GetChildLogger("events"), schema.NewEvent)

		return s.Run(ctx, sync.WithNoDelete(), sync.WithNoWarmup())
	})

	g.Go(func() error {
		s := sync.NewSync(db, factory.Core().V1().PersistentVolumeClaims().Informer(), logs.GetChildLogger("pvcs"), schema.NewPvc)

		return s.Run(ctx)
	})

	g.Go(func() error {
		s := sync.NewSync(db, factory.Core().V1().PersistentVolumes().Informer(), logs.GetChildLogger("persistent-volumes"), schema.NewPersistentVolume)

		return s.Run(ctx)
	})

	g.Go(func() error {
		s := sync.NewSync(db, factory.Batch().V1().Jobs().Informer(), logs.GetChildLogger("jobs"), schema.NewJob)

		return s.Run(ctx)
	})

	g.Go(func() error {
		s := sync.NewSync(db, factory.Batch().V1().CronJobs().Informer(), logs.GetChildLogger("cron-jobs"), schema.NewCronJob)

		return s.Run(ctx)
	})

	g.Go(func() error {
		s := sync.NewSync(db, factory.Networking().V1().Ingresses().Informer(), logs.GetChildLogger("ingresses"), schema.NewIngress)

		return s.Run(ctx)
	})

	g.Go(func() error {
		wg.Wait()

		logger.Debug("Starting multiplexers")

		return cache.Multiplexers().Run(ctx)
	})

	cleanup := kdatabase.NewCleanup(db)

	g.Go(func() error {
		return cleanup.PeriodicCleanup(ctx, kdatabase.CleanupStmt{
			Table:  "event",
			PK:     []string{"uuid"},
			Column: "created",
		})
	})

	g.Go(func() error {
		return cleanup.PeriodicCleanup(ctx, kdatabase.CleanupStmt{
			Table:  "prometheus_cluster_metric",
			PK:     []string{"cluster_uuid", "timestamp", "category", "name"},
			Column: "timestamp",
		})
	})

	g.Go(func() error {
		return cleanup.PeriodicCleanup(ctx, kdatabase.CleanupStmt{
			Table:  "prometheus_node_metric",
			PK:     []string{"node_uuid", "timestamp", "category", "name"},
			Column: "timestamp",
		})
	})

	g.Go(func() error {
		return cleanup.PeriodicCleanup(ctx, kdatabase.CleanupStmt{
			Table:  "prometheus_pod_metric",
			PK:     []string{"pod_uuid", "timestamp", "category", "name"},
			Column: "timestamp",
		})
	})

	g.Go(func() error {
		return cleanup.PeriodicCleanup(ctx, kdatabase.CleanupStmt{
			Table:  "prometheus_container_metric",
			PK:     []string{"container_uuid", "timestamp", "category", "name"},
			Column: "timestamp",
		})
	})

	// When started by systemd, NOTIFY_SOCKET is set by systemd for Type=notify
	// supervised services, which was the default setting for the Icinga for Kubernetes
	// service before switching to Type=simple. For Type=notify, we need to tell
	// systemd that Icinga for Kubernetes finished starting up.
	_ = sdnotify.Ready()

	if err := g.Wait(); err != nil {
		logger.Fatalw("Cannot run Icinga for Kubernetes", logging.Error(err))
	}
}

// removeCluster deletes the cluster with the given UUID and all data that references it from db.
func removeCluster(ctx context.Context, db *kdatabase.DB, logger *logging.Logger, clusterUuid types.UUID) error {
	type uuidColumn struct {
		Uuid types.UUID
	}

	// Check whether a cluster entry exists for the given UUID at all.
	// A missing cluster entry only produces a warning so that a rerun can
	// remove the resources an interrupted removal left behind. Data of
	// resources that are already removed, such as pod conditions, can't be
	// found anymore and stays in the database.
	selectQuery := db.Rebind(db.BuildSelectStmt(&schema.Cluster{}, uuidColumn{}) + ` WHERE "uuid" = ?`)

	var clusterUuids []types.UUID
	if err := db.SelectContext(ctx, &clusterUuids, selectQuery, clusterUuid); err != nil {
		return database.CantPerformQuery(err, selectQuery)
	}

	if len(clusterUuids) == 0 {
		logger.Warnw(
			"No cluster entry exists for UUID, trying to remove any remaining data of that cluster anyway",
			zap.String("uuid", clusterUuid.String()),
		)
	}

	// Pods don't cascade deletes to their containers, so remove the containers first.
	for _, container := range []database.Entity{&schema.Container{}, &schema.InitContainer{}, &schema.SidecarContainer{}} {
		query := db.Rebind(db.BuildSelectStmt(container, uuidColumn{}) + fmt.Sprintf(
			` WHERE "pod_uuid" IN (SELECT "uuid" FROM %q WHERE "cluster_uuid"=?)`,
			database.TableName(&schema.Pod{}),
		))

		// Scanned into []any so that the UUIDs can feed DeleteStreamed's <-chan any.
		var uuids []any
		if err := db.SelectContext(ctx, &uuids, query, clusterUuid); err != nil {
			return database.CantPerformQuery(err, query)
		}

		if err := db.DeleteStreamedCascading(ctx, container, utils.ChanFromSlice(uuids)); err != nil {
			return errors.Wrapf(err, "cannot remove rows from %q", database.TableName(container))
		}
	}

	if err := db.DeleteStreamedCascading(ctx, &schema.Cluster{}, utils.ChanFromSlice([]any{clusterUuid})); err != nil {
		return errors.Wrap(err, "cannot remove cluster rows")
	}

	logger.Infow("Removed cluster", zap.String("uuid", clusterUuid.String()))

	return nil
}

// dbHasSchema queries via db whether the database dbName has a table named "kubernetes_schema".
func dbHasSchema(db *kdatabase.DB, dbName string) (bool, error) {
	rows, err := db.Query(
		db.Rebind(`SELECT 1 FROM "INFORMATION_SCHEMA"."TABLES" WHERE "TABLE_SCHEMA"=? AND "TABLE_NAME"='kubernetes_schema'`),
		dbName,
	)
	if err != nil {
		return false, err
	}

	defer func() { _ = rows.Close() }()

	return rows.Next(), rows.Err()
}

// SyncServicePods links services to the pods their selectors match. On each
// service upsert and on each pod upsert that changes the pod's labels, it
// replaces all links of that service or pod, which also removes links that no
// longer match. On each pod or service delete, it removes all links of that
// pod or service.
func SyncServicePods(
	ctx context.Context,
	db *kdatabase.DB,
	serviceList corev1.ServiceInformer,
	podList corev1.PodInformer,
	podEvents <-chan database.Entity,
	podDeletes <-chan any,
	serviceEvents <-chan database.Entity,
	serviceDeletes <-chan any,
) error {
	if !kcache.WaitForCacheSync(ctx.Done(), serviceList.Informer().HasSynced, podList.Informer().HasSynced) {
		return ctx.Err()
	}

	// A pod's links depend only on its labels and on the services, which send
	// events of their own. Most pod events are status updates, so skipping
	// those with the labels of the last replacement saves a transaction each.
	// The map starts empty, so every pod is still replaced once on startup.
	replacedPodLabels := make(map[types.UUID]labels.Set)

	// Pod and service events share one loop so that their replacements cannot
	// interleave and delete each other's links.
	for podEvents != nil || podDeletes != nil || serviceEvents != nil || serviceDeletes != nil {
		select {
		case entity, more := <-podEvents:
			if !more {
				podEvents = nil
				continue
			}

			pod := entity.(*schema.Pod)

			// Upserts of the same pod can finish out of order, so the event may
			// carry outdated labels. Take them from the cache instead. A pod
			// that is gone or was replaced is left to its delete, which removes
			// its links. Linking it here would leave orphans behind.
			k8sPod, err := podList.Lister().Pods(pod.Namespace).Get(pod.Name)
			if kerrors.IsNotFound(err) {
				continue
			}
			if err != nil {
				return err
			}
			if k8sPod.UID != pod.Uid {
				continue
			}

			podLabels := labels.Set(k8sPod.Labels)
			if replaced, ok := replacedPodLabels[pod.Uuid]; ok && labels.Equals(replaced, podLabels) {
				continue
			}

			services, err := serviceList.Lister().Services(pod.Namespace).List(labels.NewSelector())
			if err != nil {
				return err
			}

			var servicePods []schema.ServicePod
			for _, service := range services {
				if !isSelectingService(service) {
					continue
				}

				if labels.ValidatedSetSelector(service.Spec.Selector).Matches(podLabels) {
					servicePods = append(servicePods, schema.ServicePod{
						ServiceUuid: schema.EnsureUUID(service.UID),
						PodUuid:     pod.Uuid,
					})
				}
			}

			if err := replaceServicePods(ctx, db, "pod_uuid", pod.Uuid, servicePods); err != nil {
				return err
			}

			replacedPodLabels[pod.Uuid] = podLabels
		case id, more := <-podDeletes:
			if !more {
				podDeletes = nil
				continue
			}

			podUuid := id.(types.UUID)

			// A replacement racing the cascading delete, for example through a retry,
			// can re-insert the pod's links after the cascading delete removed them.
			if err := replaceServicePods(ctx, db, "pod_uuid", podUuid, nil); err != nil {
				return err
			}

			delete(replacedPodLabels, podUuid)
		case entity, more := <-serviceEvents:
			if !more {
				serviceEvents = nil
				continue
			}

			service := entity.(*schema.Service)

			// As with pods, take the selector from the cache, not from the event.
			k8sService, err := serviceList.Lister().Services(service.Namespace).Get(service.Name)
			if kerrors.IsNotFound(err) {
				continue
			}
			if err != nil {
				return err
			}
			if k8sService.UID != service.Uid {
				continue
			}

			var servicePods []schema.ServicePod
			// A service without selectors or of type ExternalName selects no pods,
			// but its old links still have to go.
			if isSelectingService(k8sService) {
				pods, err := podList.Lister().Pods(service.Namespace).List(labels.ValidatedSetSelector(k8sService.Spec.Selector))
				if err != nil {
					return err
				}

				for _, pod := range pods {
					servicePods = append(servicePods, schema.ServicePod{
						ServiceUuid: service.Uuid,
						PodUuid:     schema.EnsureUUID(pod.UID),
					})
				}
			}

			if err := replaceServicePods(ctx, db, "service_uuid", service.Uuid, servicePods); err != nil {
				return err
			}
		case id, more := <-serviceDeletes:
			if !more {
				serviceDeletes = nil
				continue
			}

			// A pod replacement that listed the service before it left the cache
			// can re-insert its links after the cascading delete removed them.
			if err := replaceServicePods(ctx, db, "service_uuid", id.(types.UUID), nil); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// replaceServicePods replaces the service_pod rows whose column equals id with
// servicePods. It does so in one transaction, so readers never see the links
// missing in between.
func replaceServicePods(
	ctx context.Context, db *kdatabase.DB, column string, id types.UUID, servicePods []schema.ServicePod,
) error {
	deleteStmt := db.Rebind(fmt.Sprintf(`DELETE FROM %q WHERE %q = ?`, database.TableName(schema.ServicePod{}), column))
	insertStmt, placeholders := db.BuildInsertStmt(schema.ServicePod{})

	return retry.WithBackoff(
		ctx,
		func(ctx context.Context) error {
			return db.ExecTx(ctx, nil, func(ctx context.Context, tx *sqlx.Tx) error {
				if _, err := tx.ExecContext(ctx, deleteStmt, id); err != nil {
					return database.CantPerformQuery(err, deleteStmt)
				}

				for chunk := range slices.Chunk(servicePods, db.BatchSizeByPlaceholders(placeholders)) {
					if _, err := tx.NamedExecContext(ctx, insertStmt, chunk); err != nil {
						return database.CantPerformQuery(err, insertStmt)
					}
				}

				return nil
			})
		},
		retry.Retryable,
		backoff.DefaultBackoff,
		db.GetDefaultRetrySettings(),
	)
}

func isSelectingService(service *kcorev1.Service) bool {
	return len(service.Spec.Selector) > 0 && service.Spec.Type != kcorev1.ServiceTypeExternalName
}

// uuidSliceValue is a pflag.Value that collects UUIDs from a flag that may be
// repeated and may hold a comma-separated list. Invalid UUIDs are rejected
// while the flags are parsed.
type uuidSliceValue struct {
	values []types.UUID
}

// Set implements pflag.Value. It parses val as a comma-separated list of UUIDs
// and appends those not yet collected by this or previous calls.
func (s *uuidSliceValue) Set(val string) error {
	for uuidStr := range strings.SplitSeq(val, ",") {
		u, err := uuid.Parse(strings.TrimSpace(uuidStr))
		if err != nil {
			return err
		}

		uu := types.MakeUUID(u)

		if !slices.Contains(s.values, uu) {
			s.values = append(s.values, uu)
		}
	}

	return nil
}

// Type implements pflag.Value.
func (s *uuidSliceValue) Type() string {
	return "uuidSlice"
}

// String implements pflag.Value.
func (s *uuidSliceValue) String() string {
	var uuidStrs []string
	for _, u := range s.values {
		uuidStrs = append(uuidStrs, u.String())
	}

	if len(uuidStrs) == 0 {
		return ""
	}

	return "[" + strings.Join(uuidStrs, " ") + "]"
}

// Value returns a copy of the parsed UUIDs.
func (s *uuidSliceValue) Value() []types.UUID {
	return slices.Clone(s.values)
}
