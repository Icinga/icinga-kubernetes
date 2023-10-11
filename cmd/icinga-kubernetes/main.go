package main

import (
	"github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-kubernetes/internal"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
)

func main() {
	kconfig, err := kclientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		kclientcmd.NewDefaultClientConfigLoadingRules(), &kclientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		logging.Fatal(errors.Wrap(err, "can't configure Kubernetes client"))
	}

	_, err = kubernetes.NewForConfig(kconfig)
	if err != nil {
		logging.Fatal(errors.Wrap(err, "can't create Kubernetes client"))
	}

	flags, err := config.ParseFlags[internal.Flags]()
	if err != nil {
		logging.Fatal(errors.Wrap(err, "can't parse flags"))
	}

	_, err = config.FromYAMLFile[internal.Config](flags.Config)
	if err != nil {
		logging.Fatal(errors.Wrap(err, "can't create configuration"))
	}
}
