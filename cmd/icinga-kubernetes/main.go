package main

import (
	"context"
	"github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/driver"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-kubernetes/internal"
	"github.com/icinga/icinga-kubernetes/pkg/schema"
	"github.com/icinga/icinga-kubernetes/pkg/sync"
	"github.com/okzk/sdnotify"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	kinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
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

	ctx, unregisterSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer unregisterSignals()

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

	g, syncCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return sync.NewSync(
			db, schema.NewNode, informers.Core().V1().Nodes().Informer(), logs.GetChildLogger("Nodes"),
		).Run(syncCtx)
	})

	g.Go(func() error {
		return sync.NewSync(
			db, schema.NewNamespace, informers.Core().V1().Namespaces().Informer(), logs.GetChildLogger("Namespaces"),
		).Run(syncCtx)
	})

	g.Go(func() error {
		return sync.NewSync(
			db, schema.NewPod, informers.Core().V1().Pods().Informer(), logs.GetChildLogger("Pods"),
		).Run(syncCtx)
	})

	select {
	case <-ctx.Done():
		logger.Info("Shutting down")
		unregisterSignals()
	case <-syncCtx.Done():
	}

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatalf("%+v", errors.Wrap(err, "can't sync"))
	}
}
