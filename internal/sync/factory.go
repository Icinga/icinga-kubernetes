package sync

import (
	"github.com/pkg/errors"
	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Factory hands out the informers of each resource restricted to the
// namespaces to synchronize. Namespaced resources come from one factory per
// included namespace, or from a single one that filters out the excluded
// namespaces. Namespaces are filtered by name, and nodes and persistent volumes
// are never filtered.
type Factory struct {
	cluster    informers.SharedInformerFactory
	namespaces informers.SharedInformerFactory
	namespaced []informers.SharedInformerFactory
}

// NewFactory returns a Factory for the include namespaces, or for all
// namespaces but the exclude ones. At most one of them may be set. If both are
// empty, every informer sees the whole cluster.
func NewFactory(clientset kubernetes.Interface, include, exclude []string) (*Factory, error) {
	cluster := informers.NewSharedInformerFactory(clientset, 0)

	f := &Factory{
		cluster:    cluster,
		namespaces: cluster,
		namespaced: []informers.SharedInformerFactory{cluster},
	}

	var err error

	switch {
	case len(include) > 0:
		f.namespaced = nil
		for _, n := range include {
			f.namespaced = append(f.namespaced, informers.NewSharedInformerFactoryWithOptions(clientset, 0, informers.WithNamespace(n)))
		}

		f.namespaces, err = newNamespaceFactory(clientset, selection.In, include)
		if err != nil {
			return nil, errors.Wrap(err, "cannot select included namespaces")
		}
	case len(exclude) > 0:
		var byNamespace []fields.Selector
		for _, n := range exclude {
			byNamespace = append(byNamespace, fields.OneTermNotEqualSelector("metadata.namespace", n))
		}

		f.namespaced = []informers.SharedInformerFactory{
			informers.NewSharedInformerFactoryWithOptions(clientset, 0, withFieldSelector(fields.AndSelectors(byNamespace...))),
		}

		f.namespaces, err = newNamespaceFactory(clientset, selection.NotIn, exclude)
		if err != nil {
			return nil, errors.Wrap(err, "cannot select excluded namespaces")
		}
	}

	return f, nil
}

// ConfigMaps returns the informers for config maps in the selected namespaces.
func (f *Factory) ConfigMaps() Informers {
	return collect(f.namespaced, func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Core().V1().ConfigMaps().Informer()
	})
}

// CronJobs returns the informers for cron jobs in the selected namespaces.
func (f *Factory) CronJobs() Informers {
	return collect(f.namespaced, func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Batch().V1().CronJobs().Informer()
	})
}

// DaemonSets returns the informers for daemon sets in the selected namespaces.
func (f *Factory) DaemonSets() Informers {
	return collect(f.namespaced, func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Apps().V1().DaemonSets().Informer()
	})
}

// Deployments returns the informers for deployments in the selected namespaces.
func (f *Factory) Deployments() Informers {
	return collect(f.namespaced, func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Apps().V1().Deployments().Informer()
	})
}

// EndpointSlices returns the informers for endpoint slices in the selected namespaces.
func (f *Factory) EndpointSlices() Informers {
	return collect(f.namespaced, func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Discovery().V1().EndpointSlices().Informer()
	})
}

// Events returns the informers for events in the selected namespaces.
func (f *Factory) Events() Informers {
	return collect(f.namespaced, func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Events().V1().Events().Informer()
	})
}

// Ingresses returns the informers for ingresses in the selected namespaces.
func (f *Factory) Ingresses() Informers {
	return collect(f.namespaced, func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Networking().V1().Ingresses().Informer()
	})
}

// Jobs returns the informers for jobs in the selected namespaces.
func (f *Factory) Jobs() Informers {
	return collect(f.namespaced, func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Batch().V1().Jobs().Informer()
	})
}

// Namespaces returns the informer for the selected namespaces.
func (f *Factory) Namespaces() Informers {
	return Informers{f.namespaces.Core().V1().Namespaces().Informer()}
}

// Nodes returns the informer for all nodes of the cluster.
func (f *Factory) Nodes() Informers {
	return Informers{f.cluster.Core().V1().Nodes().Informer()}
}

// PersistentVolumeClaims returns the informers for persistent volume claims
// in the selected namespaces.
func (f *Factory) PersistentVolumeClaims() Informers {
	return collect(f.namespaced, func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Core().V1().PersistentVolumeClaims().Informer()
	})
}

// PersistentVolumes returns the informer for all persistent volumes of the cluster.
func (f *Factory) PersistentVolumes() Informers {
	return Informers{f.cluster.Core().V1().PersistentVolumes().Informer()}
}

// Pods returns the informers for pods in the selected namespaces.
func (f *Factory) Pods() Informers {
	return collect(f.namespaced, func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Core().V1().Pods().Informer()
	})
}

// ReplicaSets returns the informers for replica sets in the selected namespaces.
func (f *Factory) ReplicaSets() Informers {
	return collect(f.namespaced, func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Apps().V1().ReplicaSets().Informer()
	})
}

// Secrets returns the informers for secrets in the selected namespaces.
func (f *Factory) Secrets() Informers {
	return collect(f.namespaced, func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Core().V1().Secrets().Informer()
	})
}

// Services returns the informers for services in the selected namespaces.
func (f *Factory) Services() Informers {
	return collect(f.namespaced, func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Core().V1().Services().Informer()
	})
}

// StatefulSets returns the informers for stateful sets in the selected namespaces.
func (f *Factory) StatefulSets() Informers {
	return collect(f.namespaced, func(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Apps().V1().StatefulSets().Informer()
	})
}

// collect returns the informer that informer picks from each factory.
func collect(
	factories []informers.SharedInformerFactory,
	informer func(factory informers.SharedInformerFactory) cache.SharedIndexInformer,
) Informers {
	i := make(Informers, 0, len(factories))
	for _, f := range factories {
		i = append(i, informer(f))
	}

	return i
}

// newNamespaceFactory returns a factory whose informers only see the namespaces
// that op selects from names. It selects by the name label that Kubernetes sets
// on every namespace, because field selectors can only match a single name.
func newNamespaceFactory(clientset kubernetes.Interface, op selection.Operator, names []string) (informers.SharedInformerFactory, error) {
	byName, err := labels.NewRequirement(kcorev1.LabelMetadataName, op, names)
	if err != nil {
		return nil, err
	}

	return informers.NewSharedInformerFactoryWithOptions(clientset, 0, informers.WithTweakListOptions(func(o *kmetav1.ListOptions) {
		o.LabelSelector = byName.String()
	})), nil
}

// withFieldSelector restricts all informers of a factory to objects matching selector.
func withFieldSelector(selector fields.Selector) informers.SharedInformerOption {
	return informers.WithTweakListOptions(func(o *kmetav1.ListOptions) {
		o.FieldSelector = selector.String()
	})
}
