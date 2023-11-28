package main

import (
	"context"
	"github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/driver"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-kubernetes/internal"
	"github.com/icinga/icinga-kubernetes/pkg/api"
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/schema"
	"github.com/icinga/icinga-kubernetes/pkg/sync"
	"github.com/okzk/sdnotify"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	kinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
	"os"
	"os/signal"
)

func main() {
	kconfig, err := kclientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		kclientcmd.NewDefaultClientConfigLoadingRules(), &kclientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		logging.Fatal(errors.Wrap(err, "can't configure Kubernetes client"))
	}

	k, err := kubernetes.NewForConfig(kconfig)
	if err != nil {
		logging.Fatal(errors.Wrap(err, "can't create Kubernetes client"))
	}

	mk, err := metricsv.NewForConfig(kconfig)
	if err != nil {
		logging.Fatal(errors.Wrap(err, "can't create Kubernetes metrics client"))
	}

	flags, err := config.ParseFlags[internal.Flags]()
	if err != nil {
		logging.Fatal(errors.Wrap(err, "can't parse flags"))
	}

	cfg, err := config.FromYAMLFile[internal.Config](flags.Config)
	if err != nil {
		logging.Fatal(errors.Wrap(err, "can't create configuration"))
	}

	logs, err := logging.NewLoggingFromConfig("Icinga Kubernetes", &cfg.Logging)
	if err != nil {
		logging.Fatal(errors.Wrap(err, "can't configure logging"))
	}

	// Notify systemd (if supervised) that Icinga Kubernetes finished starting up.
	_ = sdnotify.Ready()

	logger := logs.GetLogger()
	defer logger.Sync()

	logger.Info("Starting up")

	driver.Register(logs.GetChildLogger("Database Driver"))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)

	db, err := database.NewDbFromConfig(&cfg.Database, logs.GetChildLogger("Database"))
	defer db.Close()
	{
		logger.Info("Connecting to database")
		err := db.PingContext(ctx)
		if err != nil {
			logger.Fatalf("%+v", errors.Wrap(err, "can't connect to database"))
		}
	}

	informers := kinformers.NewSharedInformerFactory(k, 0)

	g, ctx := errgroup.WithContext(ctx)

	// node forward channels
	forwardDeleteNodesToMetricChannel := make(chan contracts.KDelete)

	g.Go(func() error {
		return sync.NewSync(
			db,
			schema.NewNode,
			informers.Core().V1().Nodes().Informer(),
			logs.GetChildLogger("Nodes"),
		).Run(
			ctx,
			sync.WithForwardDeleteToMetric(forwardDeleteNodesToMetricChannel),
		)
	})

	g.Go(func() error {
		return sync.NewSync(
			db,
			schema.NewNamespace,
			informers.Core().V1().Namespaces().Informer(),
			logs.GetChildLogger("Namespaces"),
		).Run(ctx)
	})

	// pod forward channels
	forwardUpsertPodsToLogChannel := make(chan contracts.KUpsert)
	forwardDeletePodsToLogChannel := make(chan contracts.KDelete)

	forwardDeletePodsToMetricChannel := make(chan contracts.KDelete)

	g.Go(func() error {

		defer close(forwardUpsertPodsToLogChannel)
		defer close(forwardDeletePodsToLogChannel)

		return sync.NewSync(
			db,
			schema.NewPod,
			informers.Core().V1().Pods().Informer(),
			logs.GetChildLogger("Pods"),
		).Run(
			ctx,
			sync.WithForwardUpsertToLog(forwardUpsertPodsToLogChannel),
			sync.WithForwardDeleteToLog(forwardDeletePodsToLogChannel),
		)
	})

	// sync logs
	logSync := sync.NewLogSync(k, db, logs.GetChildLogger("ContainerLogs"))

	g.Go(func() error {
		return logSync.MaintainList(ctx, forwardUpsertPodsToLogChannel, forwardDeletePodsToLogChannel)
	})

	g.Go(func() error {
		return logSync.Run(ctx)
	})

	// sync pod and container metrics
	metricsSync := sync.NewMetricSync(mk, db, logs.GetChildLogger("Metrics"))

	g.Go(func() error {
		return metricsSync.Run(ctx)
	})

	g.Go(func() error {
		return metricsSync.Clean(ctx, forwardDeletePodsToMetricChannel)
	})

	// sync node metrics
	nodeMetricSync := sync.NewNodeMetricSync(mk, db, logs.GetChildLogger("NodeMetrics"))

	g.Go(func() error {
		return nodeMetricSync.Run(ctx)
	})

	g.Go(func() error {
		return nodeMetricSync.Clean(ctx, forwardDeleteNodesToMetricChannel)
	})

	// stream log api
	logStreamApi := api.NewLogStreamApi(k, logs.GetChildLogger("LogStreamApi"), &cfg.Api.Log)

	g.Go(func() error {
		return logStreamApi.Stream(ctx)
	})

	select {
	case <-ctx.Done():
		logger.Info("Shutting down")
		cancel()
	}

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatal(errors.Wrap(err, "can't sync"))
	}
}
