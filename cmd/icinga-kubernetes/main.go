package main

import (
	"context"
	"flag"
	_ "github.com/go-sql-driver/mysql"
	"github.com/icinga/icinga-go-library/config"
	igldatabase "github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-kubernetes/internal"
	"github.com/icinga/icinga-kubernetes/pkg/com"
	"github.com/icinga/icinga-kubernetes/pkg/daemon"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/metrics"
	"github.com/icinga/icinga-kubernetes/pkg/notifications"
	"github.com/icinga/icinga-kubernetes/pkg/periodic"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/icinga/icinga-kubernetes/pkg/sync"
	syncv1 "github.com/icinga/icinga-kubernetes/pkg/sync/v1"
	k8sMysql "github.com/icinga/icinga-kubernetes/schema/mysql"
	"github.com/pkg/errors"
	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
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

func main() {
	runtime.ReallyCrash = true

	kconfig, err := kclientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		kclientcmd.NewDefaultClientConfigLoadingRules(), &kclientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		klog.Fatal(err)
	}

	var configLocation string

	klog.InitFlags(nil)

	flag.BoolFunc("version", "print version and exit", func(_ string) error {
		internal.Version.Print()
		os.Exit(0)

		return nil
	})
	flag.StringVar(&configLocation, "config", "./config.yml", "path to the config file")
	flag.Parse()

	clientset, err := kubernetes.NewForConfig(kconfig)
	if err != nil {
		klog.Fatal(err)
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)
	log := klog.NewKlogr()

	var cfg daemon.Config
	err = config.FromYAMLFile(configLocation, &cfg)
	if err != nil {
		klog.Fatal(errors.Wrap(err, "can't create configuration"))
	}

	dbLog := log.WithName("database")
	db, err := database.NewFromConfig(&cfg.Database, dbLog)
	if err != nil {
		klog.Fatal(err)
	}
	if !db.Connect() {
		return
	}

	hasSchema, err := dbHasSchema(db, cfg.Database.Database)
	if err != nil {
		klog.Fatal(err)
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

	var nclient *notifications.Client
	if err := notifications.SyncSourceConfig(context.Background(), db, &cfg.Notifications); err != nil {
		klog.Fatal(err)
	}
	if cfg.Notifications.Url != "" {
		nclient = notifications.NewClient(db, cfg.Notifications)
	}

	g, ctx := errgroup.WithContext(context.Background())

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
		promMetricSync := metrics.NewPromMetricSync(promApiClient, db2)

		g.Go(func() error {
			return promMetricSync.Nodes(ctx, factory.Core().V1().Nodes().Informer())
		})

		g.Go(func() error {
			return promMetricSync.Pods(ctx, factory.Core().V1().Pods().Informer())

			//return promMetricSync.Run(ctx)
		})
	}

	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Core().V1().Namespaces().Informer(), log.WithName("namespaces"), schemav1.NewNamespace)

		return s.Run(ctx)
	})
	g.Go(func() error {
		nodes := internal.NewMultiplex()
		if cfg.Notifications.Url != "" {
			nodesOut := nodes.Out()
			g.Go(func() error { return nclient.Stream(ctx, nodesOut) })
		}

		nodesIn := nodes.In()
		g.Go(func() error { return nodes.Do(ctx) })

		s := syncv1.NewSync(db, factory.Core().V1().Nodes().Informer(), log.WithName("nodes"), schemav1.NewNode)
		return s.Run(ctx, sync.WithOnUpsert(com.ForwardBulk(nodesIn)))
	})
	g.Go(func() error {
		pods := internal.NewMultiplex()
		deletedPodUuids := internal.NewMultiplex()

		if cfg.Notifications.Url != "" {
			podsOut := pods.Out()
			g.Go(func() error { return nclient.Stream(ctx, podsOut) })
		}

		schemav1.SyncContainers(ctx, db, g, pods.Out(), deletedPodUuids.Out())

		f := schemav1.NewPodFactory(clientset)
		s := syncv1.NewSync(db, factory.Core().V1().Pods().Informer(), log.WithName("pods"), f.New)

		podsIn := pods.In()
		deletedIn := deletedPodUuids.In()

		g.Go(func() error { return pods.Do(ctx) })
		g.Go(func() error { return deletedPodUuids.Do(ctx) })

		return s.Run(ctx, sync.WithOnUpsert(com.ForwardBulk(podsIn)), sync.WithOnDelete(com.ForwardBulk(deletedIn)))
	})
	g.Go(func() error {
		deployments := internal.NewMultiplex()
		if cfg.Notifications.Url != "" {
			deploymentsOut := deployments.Out()
			g.Go(func() error { return nclient.Stream(ctx, deploymentsOut) })
		}
		s := syncv1.NewSync(db, factory.Apps().V1().Deployments().Informer(), log.WithName("deployments"), schemav1.NewDeployment)

		deploymentsIn := deployments.In()
		g.Go(func() error { return deployments.Do(ctx) })

		return s.Run(ctx, sync.WithOnUpsert(com.ForwardBulk(deploymentsIn)))
	})
	g.Go(func() error {
		daemonSet := internal.NewMultiplex()
		if cfg.Notifications.Url != "" {
			daemonSetOut := daemonSet.Out()
			g.Go(func() error { return nclient.Stream(ctx, daemonSetOut) })
		}

		daemonSetIn := daemonSet.In()
		g.Go(func() error { return daemonSet.Do(ctx) })

		s := syncv1.NewSync(db, factory.Apps().V1().DaemonSets().Informer(), log.WithName("daemon-sets"), schemav1.NewDaemonSet)

		return s.Run(ctx, sync.WithOnUpsert(com.ForwardBulk(daemonSetIn)))
	})
	g.Go(func() error {
		replicaSet := internal.NewMultiplex()
		if cfg.Notifications.Url != "" {
			replicaSetOut := replicaSet.Out()
			g.Go(func() error { return nclient.Stream(ctx, replicaSetOut) })
		}

		replicaSetIn := replicaSet.In()
		g.Go(func() error { return replicaSet.Do(ctx) })

		s := syncv1.NewSync(db, factory.Apps().V1().ReplicaSets().Informer(), log.WithName("replica-sets"), schemav1.NewReplicaSet)

		return s.Run(ctx, sync.WithOnUpsert(com.ForwardBulk(replicaSetIn)))
	})
	g.Go(func() error {
		statefulSet := internal.NewMultiplex()
		if cfg.Notifications.Url != "" {
			statefulSetOut := statefulSet.Out()
			g.Go(func() error { return nclient.Stream(ctx, statefulSetOut) })
		}

		statefulSetIn := statefulSet.In()
		g.Go(func() error { return statefulSet.Do(ctx) })

		s := syncv1.NewSync(db, factory.Apps().V1().StatefulSets().Informer(), log.WithName("stateful-sets"), schemav1.NewStatefulSet)

		return s.Run(ctx, sync.WithOnUpsert(com.ForwardBulk(statefulSetIn)))
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

	errs := make(chan error, 1)
	defer close(errs)
	defer periodic.Start(ctx, time.Hour, func(tick periodic.Tick) {
		olderThan := tick.Time.AddDate(0, 0, -1)

		_, err := db.CleanupOlderThan(
			ctx, database.CleanupStmt{
				Table:  "event",
				PK:     "uuid",
				Column: "created",
			}, 5000, olderThan,
		)
		if err != nil {
			select {
			case errs <- err:
			case <-ctx.Done():
			}

			return
		}
	}, periodic.Immediate()).Stop()
	com.ErrgroupReceive(ctx, g, errs)

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
