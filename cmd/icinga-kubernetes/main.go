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
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
)

func main() {
	var configPath string
	pflag.StringVarP(&configPath, "config", "c", "./config.yml", "path to config file")

	kconfigOverrides := &kclientcmd.ConfigOverrides{}
	kclientcmd.BindOverrideFlags(kconfigOverrides, pflag.CommandLine, kclientcmd.RecommendedConfigOverrideFlags(""))

	kclientconfig := kclientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		kclientcmd.NewDefaultClientConfigLoadingRules(), kconfigOverrides)

	pflag.Parse()

	kconfig, err := kclientconfig.ClientConfig()
	if err != nil {
		logging.Fatal(errors.Wrap(err, "can't configure Kubernetes client"))
	}

	k, err := kubernetes.NewForConfig(kconfig)
	if err != nil {
		logging.Fatal(errors.Wrap(err, "can't create Kubernetes client"))
	}

	namespace, overridden, err := kclientconfig.Namespace()
	if err != nil {
		logging.Fatal(errors.Wrap(err, "can't get namespace from CLI"))
	} else if !overridden {
		namespace = kmetav1.NamespaceAll
	}

	cfg, err := config.FromYAMLFile[internal.Config](configPath)
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

	ctx := context.Background()

	db, err := database.NewDbFromConfig(&cfg.Database, logs.GetChildLogger("Database"))
	defer db.Close()
	{
		logger.Info("Connecting to database")
		err := db.PingContext(ctx)
		if err != nil {
			logger.Fatalf("%+v", errors.Wrap(err, "can't connect to database"))
		}
	}

	informers := kinformers.NewSharedInformerFactoryWithOptions(k, 0, kinformers.WithNamespace(namespace))

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return sync.NewSync(
			db, schema.NewNode, informers.Core().V1().Nodes().Informer(), logs.GetChildLogger("Nodes"),
		).Run(ctx, namespace)
	})

	g.Go(func() error {
		return sync.NewSync(
			db, schema.NewNamespace, informers.Core().V1().Namespaces().Informer(), logs.GetChildLogger("Namespaces"),
		).Run(ctx, namespace)
	})

	g.Go(func() error {
		return sync.NewSync(
			db, schema.NewPod, informers.Core().V1().Pods().Informer(), logs.GetChildLogger("Pods"),
		).Run(ctx, namespace)
	})

	if err := g.Wait(); err != nil {
		logging.Fatal(errors.Wrap(err, "can't sync"))
	}
}
