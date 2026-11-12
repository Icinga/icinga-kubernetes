package sync

import (
	"context"
	"fmt"
	"slices"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal/kdatabase"
	"github.com/icinga/icinga-kubernetes/internal/schema"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
	kcorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	kcache "k8s.io/client-go/tools/cache"
)

// ServicePodSync syncs the service_pod links between services and the pods
// their selectors match.
type ServicePodSync struct {
	db               *kdatabase.DB
	serviceInformers Informers
	podInformers     Informers
	log              *logging.Logger
	podEvents        <-chan database.Entity
	podDeletes       <-chan any
	serviceEvents    <-chan database.Entity
	serviceDeletes   <-chan any
}

// NewServicePodSync returns a ServicePodSync that reads services and pods from
// the given informers and handles the pod and service upserts and deletes sent
// on the given channels.
func NewServicePodSync(
	db *kdatabase.DB,
	serviceInformers Informers,
	podInformers Informers,
	log *logging.Logger,
	podEvents <-chan database.Entity,
	podDeletes <-chan any,
	serviceEvents <-chan database.Entity,
	serviceDeletes <-chan any,
) *ServicePodSync {
	return &ServicePodSync{
		db:               db,
		serviceInformers: serviceInformers,
		podInformers:     podInformers,
		log:              log,
		podEvents:        podEvents,
		podDeletes:       podDeletes,
		serviceEvents:    serviceEvents,
		serviceDeletes:   serviceDeletes,
	}
}

// Run links services to the pods their selectors match. On each service upsert
// and on each pod upsert that changes the pod's labels, it replaces all links
// of that service or pod, which also removes links that no longer match.
// On each pod or service delete, it removes all links of that pod or service.
func (s *ServicePodSync) Run(ctx context.Context) error {
	if !kcache.WaitForCacheSync(ctx.Done(), s.serviceInformers.HasSynced, s.podInformers.HasSynced) {
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
			podName := zap.String("pod", kcache.NewObjectName(pod.Namespace, pod.Name).String())

			// Upserts of the same pod can finish out of order, so the event may
			// carry outdated labels. Take them from the cache instead. A pod
			// that is gone or was replaced is left to its delete, which removes
			// its links. Linking it here would leave orphans behind.
			obj, exists, err := s.podInformers.GetByKey(kcache.NewObjectName(pod.Namespace, pod.Name).String())
			if err != nil {
				return err
			}
			if !exists {
				s.log.Debugw("Skipping links of pod missing from cache", podName)

				continue
			}
			k8sPod := obj.(*kcorev1.Pod)
			if k8sPod.UID != pod.Uid {
				s.log.Debugw("Skipping links of recreated pod", podName)

				continue
			}

			podLabels := labels.Set(k8sPod.Labels)
			if replaced, ok := replacedPodLabels[pod.Uuid]; ok && labels.Equals(replaced, podLabels) {
				s.log.Debugw("Skipping links of pod with unchanged labels", podName)

				continue
			}

			services, err := s.serviceInformers.ListByNamespace(pod.Namespace, labels.Everything())
			if err != nil {
				return err
			}

			var servicePods []schema.ServicePod
			for _, item := range services {
				service := item.(*kcorev1.Service)
				if !isSelectingService(service) {
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

			s.log.Debugw("Replaced links of pod", podName, zap.Int("services", len(servicePods)))

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

			s.log.Debugw("Removed links of deleted pod", zap.String("uuid", podUuid.String()))

			delete(replacedPodLabels, podUuid)
		case entity, more := <-s.serviceEvents:
			if !more {
				s.serviceEvents = nil
				continue
			}

			service := entity.(*schema.Service)
			serviceName := zap.String("service", kcache.NewObjectName(service.Namespace, service.Name).String())

			// As with pods, take the selector from the cache, not from the event.
			obj, exists, err := s.serviceInformers.GetByKey(kcache.NewObjectName(service.Namespace, service.Name).String())
			if err != nil {
				return err
			}
			if !exists {
				s.log.Debugw("Skipping links of service missing from cache", serviceName)

				continue
			}
			k8sService := obj.(*kcorev1.Service)
			if k8sService.UID != service.Uid {
				s.log.Debugw("Skipping links of recreated service", serviceName)

				continue
			}

			var servicePods []schema.ServicePod
			// A service without selectors or of type ExternalName selects no pods,
			// but its old links still have to go.
			if isSelectingService(k8sService) {
				pods, err := s.podInformers.ListByNamespace(service.Namespace, labels.ValidatedSetSelector(k8sService.Spec.Selector))
				if err != nil {
					return err
				}

				for _, item := range pods {
					pod := item.(*kcorev1.Pod)
					servicePods = append(servicePods, schema.ServicePod{
						ServiceUuid: service.Uuid,
						PodUuid:     schema.EnsureUUID(pod.UID),
					})
				}
			}

			if err := replaceServicePods(ctx, s.db, "service_uuid", service.Uuid, servicePods); err != nil {
				return err
			}

			s.log.Debugw("Replaced links of service", serviceName, zap.Int("pods", len(servicePods)))
		case id, more := <-s.serviceDeletes:
			if !more {
				s.serviceDeletes = nil
				continue
			}

			serviceUuid := id.(types.UUID)

			// A pod replacement that listed the service before it left the cache
			// can re-insert its links after the cascading delete removed them.
			if err := replaceServicePods(ctx, s.db, "service_uuid", serviceUuid, nil); err != nil {
				return err
			}

			s.log.Debugw("Removed links of deleted service", zap.String("uuid", serviceUuid.String()))
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
			return db.ExecTx(ctx, nil, func(ctx context.Context, tx *sqlx.Tx) error {
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

func isSelectingService(service *kcorev1.Service) bool {
	return len(service.Spec.Selector) > 0 && service.Spec.Type != kcorev1.ServiceTypeExternalName
}
