package sync

import (
	"context"
	"fmt"
	"slices"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal/kdatabase"
	"github.com/icinga/icinga-kubernetes/internal/schema"
	"github.com/jmoiron/sqlx"
	kcorev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	kcache "k8s.io/client-go/tools/cache"
)

// ServicePodSync syncs the service_pod links between services and the pods
// their selectors match.
type ServicePodSync struct {
	db             *kdatabase.DB
	serviceList    informerscorev1.ServiceInformer
	podList        informerscorev1.PodInformer
	podEvents      <-chan database.Entity
	podDeletes     <-chan any
	serviceEvents  <-chan database.Entity
	serviceDeletes <-chan any
}

// NewServicePodSync returns a ServicePodSync that reads services and pods from
// the given informers and handles the pod and service upserts and deletes sent
// on the given channels.
func NewServicePodSync(
	db *kdatabase.DB,
	serviceList informerscorev1.ServiceInformer,
	podList informerscorev1.PodInformer,
	podEvents <-chan database.Entity,
	podDeletes <-chan any,
	serviceEvents <-chan database.Entity,
	serviceDeletes <-chan any,
) *ServicePodSync {
	return &ServicePodSync{
		db:             db,
		serviceList:    serviceList,
		podList:        podList,
		podEvents:      podEvents,
		podDeletes:     podDeletes,
		serviceEvents:  serviceEvents,
		serviceDeletes: serviceDeletes,
	}
}

// Run links services to the pods their selectors match. On each
// service upsert and on each pod upsert that changes the pod's labels, it
// replaces all links of that service or pod, which also removes links that no
// longer match. On each pod or service delete, it removes all links of that
// pod or service.
func (s *ServicePodSync) Run(ctx context.Context) error {
	if !kcache.WaitForCacheSync(ctx.Done(), s.serviceList.Informer().HasSynced, s.podList.Informer().HasSynced) {
		return ctx.Err()
	}

	// A pod's links depend only on its labels and on the services, which send
	// events of their own. Most pod events are status updates, so skipping
	// those with the labels of the last replacement saves a transaction each.
	// The map starts empty, so every pod is still replaced once on startup.
	replacedPodLabels := make(map[types.UUID]labels.Set)

	// Pod and service events share one loop so that their replacements cannot
	// interleave and delete each other's links.
	for s.podEvents != nil || s.podDeletes != nil || s.serviceEvents != nil || s.serviceDeletes != nil {
		select {
		case entity, more := <-s.podEvents:
			if !more {
				s.podEvents = nil
				continue
			}

			pod := entity.(*schema.Pod)

			// Upserts of the same pod can finish out of order, so the event may
			// carry outdated labels. Take them from the cache instead. A pod
			// that is gone or was replaced is left to its delete, which removes
			// its links. Linking it here would leave orphans behind.
			k8sPod, err := s.podList.Lister().Pods(pod.Namespace).Get(pod.Name)
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

			services, err := s.serviceList.Lister().Services(pod.Namespace).List(labels.NewSelector())
			if err != nil {
				return err
			}

			var servicePods []schema.ServicePod
			for _, service := range services {
				if len(service.Spec.Selector) == 0 || service.Spec.Type == kcorev1.ServiceTypeExternalName {
					continue
				}

				if labels.ValidatedSetSelector(service.Spec.Selector).Matches(podLabels) {
					servicePods = append(servicePods, schema.ServicePod{
						ServiceUuid: schema.EnsureUUID(service.UID),
						PodUuid:     pod.Uuid,
					})
				}
			}

			if err := replaceServicePods(ctx, s.db, "pod_uuid", pod.Uuid, servicePods); err != nil {
				return err
			}

			replacedPodLabels[pod.Uuid] = podLabels
		case id, more := <-s.podDeletes:
			if !more {
				s.podDeletes = nil
				continue
			}

			podUuid := id.(types.UUID)

			// A replacement racing the cascading delete, for example through a retry,
			// can re-insert the pod's links after the cascading delete removed them.
			if err := replaceServicePods(ctx, s.db, "pod_uuid", podUuid, nil); err != nil {
				return err
			}

			delete(replacedPodLabels, podUuid)
		case entity, more := <-s.serviceEvents:
			if !more {
				s.serviceEvents = nil
				continue
			}

			service := entity.(*schema.Service)

			// As with pods, take the selector from the cache, not from the event.
			k8sService, err := s.serviceList.Lister().Services(service.Namespace).Get(service.Name)
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
			if len(k8sService.Spec.Selector) > 0 && k8sService.Spec.Type != kcorev1.ServiceTypeExternalName {
				pods, err := s.podList.Lister().Pods(service.Namespace).List(labels.ValidatedSetSelector(k8sService.Spec.Selector))
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

			if err := replaceServicePods(ctx, s.db, "service_uuid", service.Uuid, servicePods); err != nil {
				return err
			}
		case id, more := <-s.serviceDeletes:
			if !more {
				s.serviceDeletes = nil
				continue
			}

			// A pod replacement that listed the service before it left the cache
			// can re-insert its links after the cascading delete removed them.
			if err := replaceServicePods(ctx, s.db, "service_uuid", id.(types.UUID), nil); err != nil {
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
			return db.ExecTx(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
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
