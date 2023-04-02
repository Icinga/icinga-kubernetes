/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/jmoiron/sqlx"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"time"
)

// Controller demonstrates how to implement a controller with client-go.
type Controller struct {
	indexer  cache.Indexer
	queue    workqueue.RateLimitingInterface
	informer cache.Controller
	db       *sqlx.DB
}

// NewController creates a new Controller.
func NewController(queue workqueue.RateLimitingInterface, indexer cache.Indexer, informer cache.Controller, db *sqlx.DB) *Controller {
	return &Controller{
		informer: informer,
		indexer:  indexer,
		queue:    queue,
		db:       db,
	}
}

func (c *Controller) processNextItem() bool {
	// Wait until there is a new item in the working queue.
	// `shutdown` is true if `ShutDown()` was called on the queue as in `Controller::Run()`.
	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two pods with the same key are never processed in
	// parallel.
	defer c.queue.Done(key)

	// Invoke the method containing the business logic.
	// Note that the `.(string)` type assertion is safe because
	// only strings are added to the queue.
	err := c.syncToDb(key.(string))
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, key)

	return true
}

// syncToDb is the business logic of the controller.
// It prints information about the pod to stdout and issues database statements.
// In case an error happened, it simply returns the error.
// The retry logic should not be part of the business logic.
func (c *Controller) syncToDb(key string) error {
	// Get the pod for the given key.
	// If the pod no longer exists, exists is false.
	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v", key, err)

		return err
	}

	if !exists {
		fmt.Printf("Pod %s does not exist anymore\n", key)

		// TODO: Issue DELETE statement.
	} else {
		fmt.Printf("Sync/Add/Update for Pod %s\n", obj.(*v1.Pod).GetName())

		// TODO: Issue INSERT INTO ... ON DUPLICATE KEY UPDATE statement.
		// Note that at the moment we issue an upsert statement that handles both inserts and updates.
		// This way we don't have to keep separate information about what has already been synchronized and
		// in which state it is, and this statement will be used later for bulk updates,
		// i.e. the statement is sent once and the values are streamed to the database,
		// which is much faster than issuing statements one after the other.
		pod, err := schemav1.NewPodFromK8s(obj.(*v1.Pod))
		if err != nil {
			return err
		}
		stmt := `INSERT INTO pod (name)
VALUES (:name)
ON DUPLICATE KEY UPDATE name = VALUES(NAME)`
		fmt.Printf("%+v\n", pod)
		_, err = c.db.NamedExecContext(context.TODO(), stmt, pod)
		if err != nil {
			return err
		}
	}

	return nil
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.queue.Forget(key)

		return
	}

	// This controller retries 5 times if something goes wrong. After that, it stops trying.
	if numTries := c.queue.NumRequeues(key); numTries < 5 {
		klog.Infof("%d/%d Error syncing pod %v: %v", numTries+1, 5, key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.queue.AddRateLimited(key)

		return
	}

	c.queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	runtime.HandleError(err)
	klog.Infof("Dropping pod %q out of the queue: %v", key, err)
}

// Run begins watching and syncing.
func (c *Controller) Run(workers int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()
	klog.Info("Starting Pod controller")

	go c.informer.Run(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))

		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	klog.Info("Stopping Pod controller")
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

func main() {
	var kubeconfig string
	var master string

	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
	flag.Parse()

	// creates the connection config
	config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		klog.Fatal(err)
	}

	// creates the clientset for accessing the various Kubernetes API groups and resources
	// https://kubernetes.io/docs/concepts/overview/kubernetes-api/#api-groups-and-versioning
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	// create the pod watcher to track changes to pods, i.e.create, delete and update operations
	// https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes
	podListWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "pods", v1.NamespaceDefault, fields.Everything())

	// create the workqueue which in the current state of the code contains
	// only a sequence of keys of the pods for which an operation was made,
	// regardless of whether they were created, updated or deleted
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// Bind the workqueue to a cache with the help of an informer. This way we make sure that
	// whenever the cache is updated, the pod key is added to the workqueue.
	// Note that when we finally process the item from the workqueue, we might see a newer version
	// of the Pod than the version which was responsible for triggering the update.
	indexer, informer := cache.NewIndexerInformer(podListWatcher, &v1.Pod{}, 0, cache.ResourceEventHandlerFuncs{
		// The informer knows if a resource has been added, updated or deleted because it maintains a cache.
		// When the cache is empty, there are only add messages.
		// If the cache was filled before the initial sync,
		// there are add messages for new pods,
		// update messages for existing pods,
		// and delete messages for removed pods.
		// Once the cache is filled,
		// add, update, and delete notifications are issued as the cluster evolves.
		// Here we just add the pod key to our workqueue for later processing.
		// There is no distinction between message types -
		// see Controller::syncToDb() for how we handle the queue.
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			// IndexerInformer uses a delta queue, therefore for deletes we have to use this
			// key function.
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	}, cache.Indexers{})

	// TODO: Create database from a YAML configuration file.
	db, err := sqlx.Open("mysql", "/kubernetes")
	if err != nil {
		klog.Fatal(err)
	}

	controller := NewController(queue, indexer, informer, db)

	// TODO: Warm up the cache for initial synchronization.
	// Pods that have been already persisted to database must be added to the cache.
	// Instead of adding an actual pod object to the cache,
	// just add its key with cache.ExplicitKey(),
	// where the key has the format namespace/name.
	// This way we only need to load minimal information from the database,
	// and this applies to all resource types, not just pods.
	indexer.Add(cache.ExplicitKey(v1.NamespaceDefault + "/" + "fake-pod"))

	// Now let's start the controller
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)

	// Wait forever
	select {}
}
