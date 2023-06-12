package main

import (
	"context"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/icinga/icinga-kubernetes/pkg/com"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/icinga/icinga-kubernetes/pkg/sync"
	syncv1 "github.com/icinga/icinga-kubernetes/pkg/sync/v1"
	"golang.org/x/sync/errgroup"
	kcorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	kmetrics "k8s.io/metrics/pkg/client/clientset/versioned"
	"os"
	"path/filepath"
	"time"
)

func main() {
	runtime.ReallyCrash = true

	var config string
	var kubeconfig string
	var master string

	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("error getting user home dir: %v\n", err)
		os.Exit(1)
	}
	kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
	klog.InitFlags(nil)
	flag.StringVar(&kubeconfig, "kubeconfig", kubeConfigPath, "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
	flag.StringVar(&config, "config", "./config.yml", "path to the config file")
	flag.Parse()

	clientconfig, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		klog.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(clientconfig)
	if err != nil {
		klog.Fatal(err)
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)
	log := klog.NewKlogr()

	// eventsCtx, cancelEventsCtx := context.WithCancel(context.Background())
	// eventsInformer := factory.Events().V1().Events().Informer()
	// go func() {
	// 	eventsInformer.Run(eventsCtx.Done())
	// 	fmt.Println("Informer done.")
	// }()
	//
	// if !cache.WaitForCacheSync(eventsCtx.Done(), eventsInformer.HasSynced) {
	// 	panic("timed out waiting for caches to sync")
	// }
	// cancelEventsCtx()
	//
	// fmt.Println(eventsInformer.HasSynced(), eventsInformer.GetIndexer().List())
	// return

	// metrics, err := kmetrics.NewForConfig(clientconfig)
	// if err != nil {
	// 	klog.Fatal(err)
	// }
	//
	// nodeMetricsInformer := NewNodeMetricsInformer(metrics, 0)
	// nodeMetricsInformerHasSynced := nodeMetricsInformer.Informer().HasSynced
	// ctx := context.Background()
	// go nodeMetricsInformer.Informer().Run(ctx.Done())
	//
	// if ok := cache.WaitForCacheSync(ctx.Done(), nodeMetricsInformerHasSynced); !ok {
	// 	panic("metrics resources failed to sync [nodes, pods, containers]")
	// }
	//
	// fmt.Println(nodeMetricsInformer.Lister().List(labels.Everything()))
	// return

	// watch, err := metrics.MetricsV1beta1().PodMetricses("default").Watch(context.TODO(), kmetav1.ListOptions{})
	// if err != nil {
	// 	klog.Fatal(err)
	// }
	// for {
	// 	select {
	// 	case event, more := <-watch.ResultChan():
	// 		if !more {
	// 			return
	// 		}
	//
	// 		fmt.Println(event)
	// 	}
	// }
	// return

	d, err := database.FromYAMLFile(config)
	if err != nil {
		klog.Fatal(err)
	}
	db, err := database.NewFromConfig(d, log.WithName("database"))
	if err != nil {
		klog.Fatal(err)
	}
	if !db.Connect() {
		return
	}

	g, ctx := errgroup.WithContext(context.Background())
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

		g.Go(func() error {
			defer runtime.HandleCrash()
			defer close(pods)

			for {
				select {
				case _, more := <-pods:
					if !more {
						return nil
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		f := schemav1.NewPodFactory(clientset)
		s := syncv1.NewSync(db, factory.Core().V1().Pods().Informer(), log.WithName("pods"), f.New)

		return s.Run(ctx, sync.WithOnUpsert(com.ForwardBulk(pods)))
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
		s := syncv1.NewSync(db, factory.Events().V1().Events().Informer(), log.WithName("events"), schemav1.NewEvent)

		return s.Run(ctx, sync.WithNoDelete(), sync.WithNoWarumup())
	})
	g.Go(func() error {
		s := syncv1.NewSync(db, factory.Core().V1().PersistentVolumeClaims().Informer(), log.WithName("pvcs"), schemav1.NewPvc)

		return s.Run(ctx)
	})
	if err := g.Wait(); err != nil {
		klog.Fatal(err)
	}
}

type NodeMetricsLister struct {
	indexer cache.Indexer
}

func NewNodeMetricsLister(indexer cache.Indexer) *NodeMetricsLister {
	return &NodeMetricsLister{indexer: indexer}
}

func (s *NodeMetricsLister) List(selector labels.Selector) (ret []*v1beta1.NodeMetrics, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.NodeMetrics))
	})
	return ret, err
}

func (s *NodeMetricsLister) Get(name string) (*v1beta1.NodeMetrics, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(kcorev1.Resource("nodemetrics"), name)
	}
	return obj.(*v1beta1.NodeMetrics), nil
}

type NodeMetricsInformer struct {
	client   kmetrics.Interface
	informer cache.SharedIndexInformer
	lister   *NodeMetricsLister
}

func NewNodeMetricsInformer(client kmetrics.Interface, resyncPeriod time.Duration) *NodeMetricsInformer {
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options kmetav1.ListOptions) (kruntime.Object, error) {
				return client.MetricsV1beta1().NodeMetricses().List(context.TODO(), options)
			},
			WatchFunc: func(options kmetav1.ListOptions) (watch.Interface, error) {

				return client.MetricsV1beta1().NodeMetricses().Watch(context.TODO(), options)
			},
		},
		&v1beta1.NodeMetrics{},
		resyncPeriod,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	return &NodeMetricsInformer{client: client, informer: informer}
}

func (i *NodeMetricsInformer) Informer() cache.SharedIndexInformer {
	return i.informer
}

func (i *NodeMetricsInformer) Lister() *NodeMetricsLister {
	if i.lister != nil {
		return i.lister
	}
	i.lister = NewNodeMetricsLister(i.informer.GetIndexer())
	return i.lister
}

// func GetMetrics(pod *schemav1.Pod, containerMetrics v1beta1.ContainerMetrics) error {
// 	metrics, err := p.metricsClientset.MetricsV1beta1().PodMetricses(pod.Namespace).Get(context.TODO(), pod.Name,
// 		metav1.GetOptions{})
// 	if err != nil {
// 		return err
// 	}
// 	cpuUsage, memoryUsage, storageUsage, ephemeralStorageUsage, err := p.GetPodMetrics(pod)
// 	if err != nil {
// 		return err
// 	}
//
// 	podMetrics := schemav1.PodMetrics{
// 		Namespace:             pod.Namespace,
// 		PodName:               pod.Name,
// 		ContainerName:         containerMetrics.Name,
// 		Timestamp:             types.UnixMilli(metrics.Timestamp.Time),
// 		Duration:              metrics.Window.Duration,
// 		CPUUsage:              cpuUsage,
// 		MemoryUsage:           memoryUsage,
// 		StorageUsage:          storageUsage,
// 		EphemeralStorageUsage: ephemeralStorageUsage,
// 	}
//
// 	stmt := database.BuildUpsertStmt(podMetrics)
// 	_, err = p.db.NamedExecContext(context.TODO(), stmt, podMetrics)
// 	if err != nil {
// 		return err
// 	}
//
// 	return nil
// }
