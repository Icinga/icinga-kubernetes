package controller

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"time"
)

// Controller demonstrates how to implement a controller with client-go.
type Controller struct {
	queue    workqueue.RateLimitingInterface
	informer cache.SharedIndexInformer
	syncFn   func(key string, obj interface{}, exists bool) error
}

// NewController creates a new Controller.
func NewController(
	informer cache.SharedIndexInformer,
	syncFn func(key string, obj interface{}, exists bool) error,
) *Controller {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	_, err := informer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
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
	})
	if err != nil {
		panic(err)
	}

	return &Controller{
		informer: informer,
		queue:    queue,
		syncFn:   syncFn,
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
	err := c.sync(key.(string))
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, key)

	return true
}

// syncToDb is the business logic of the controller.
// It prints information about the pod to stdout and issues database statements.
// In case an error happened, it simply returns the error.
// The retry logic should not be part of the business logic.
func (c *Controller) sync(key string) error {
	// Get the pod for the given key.
	// If the pod no longer exists, exists is false.
	obj, exists, err := c.informer.GetIndexer().GetByKey(key)
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v", key, err)

		return err
	}

	return c.syncFn(key, obj, exists)
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
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))

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
