package controller

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	"github.com/jmoiron/sqlx"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
	"log"
	"reflect"
	"sync"
	"time"
)

type PodSync struct {
	clientset        *kubernetes.Clientset
	metricsClientset *metricsv.Clientset
	db               *sqlx.DB
	knownPods        map[string]*schemav1.Pod
	mu               sync.Mutex
}

func NewPodSync(clientset *kubernetes.Clientset, metricsClientset *metricsv.Clientset, db *sqlx.DB) *PodSync {
	return &PodSync{
		clientset:        clientset,
		metricsClientset: metricsClientset,
		db:               db,
		knownPods:        make(map[string]*schemav1.Pod),
	}
}

func (p *PodSync) SyncMetrics(ctx context.Context, interval time.Duration) error {
	for {
		p.mu.Lock()
		for _, pod := range p.knownPods {
			podMetrics, _ := p.metricsClientset.MetricsV1beta1().PodMetricses(pod.Namespace).Get(context.TODO(), pod.Name,
				metav1.GetOptions{})
			for _, container := range podMetrics.Containers {
				if err := p.syncPodMetrics(pod, container); err != nil {
					return err
				}
			}
		}

		p.mu.Unlock()
		time.Sleep(interval)
	}
}

func (p *PodSync) GetPodMetrics(pod *schemav1.Pod) (cpu float64, memory float64, storage float64, estorage float64, err error) {
	podMetrics, err := p.metricsClientset.MetricsV1beta1().PodMetricses(pod.Namespace).Get(context.TODO(), pod.Name,
		metav1.GetOptions{})
	if err != nil {
		return
	}

	for _, container := range podMetrics.Containers {
		cpuValue := container.Usage[corev1.ResourceCPU]
		cpuInt64 := cpuValue.AsDec().UnscaledBig().Int64()
		cpu = float64(cpuInt64) / 1000.0

		memoryValue := container.Usage[corev1.ResourceMemory]
		memoryInt64 := memoryValue.Value()
		memory = float64(memoryInt64) / (1024 * 1024)

		storageValue := container.Usage[corev1.ResourceStorage]
		storageInt64 := storageValue.Value()
		storage = float64(storageInt64) / (1024 * 1024)

		estorageValue := container.Usage[corev1.ResourceEphemeralStorage]
		estorageInt64 := estorageValue.Value()
		estorage = float64(estorageInt64) / (1024 * 1024)
	}

	return
}

func (p *PodSync) Sync(key string, obj interface{}, exists bool) error {
	if !exists {
		fmt.Printf("Pod %s does not exist anymore\n", key)

		namespace, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return err
		}

		p.mu.Lock()
		delete(p.knownPods, key)
		p.mu.Unlock()

		_, err = p.db.Exec(`DELETE FROM pod WHERE namespace=? AND name=?`, namespace, name)
		if err != nil {
			return err
		}

		_, err = p.db.Exec(`DELETE FROM container_logs WHERE namespace=? AND pod_name=?`, namespace, name)
		if err != nil {
			return err
		}

		_, err = p.db.Exec(`DELETE FROM pod_metrics WHERE namespace=? AND pod_name=?`, namespace, name)
		if err != nil {
			return err
		}

		_, err = p.db.Exec(`DELETE FROM volumes WHERE namespace=? AND pod_name=?`, namespace, name)
		if err != nil {
			return err
		}

		_, err = p.db.Exec(`DELETE FROM pod_pvc WHERE namespace=? AND pod_name=?`, namespace, name)
		if err != nil {
			return err
		}

		_, err = p.db.Exec(`DELETE FROM container_volume_mount WHERE namespace=? AND pod_name=?`, namespace, name)
		if err != nil {
			return err
		}
	} else {
		fmt.Printf("Sync/Add/Update for Pod %s\n", obj.(*corev1.Pod).GetName())
		pod, err := schemav1.NewPodFromK8s(obj.(*corev1.Pod))
		if err != nil {
			return err
		}

		p.mu.Lock()
		p.knownPods[key] = pod
		p.mu.Unlock()

		stmt := `INSERT INTO pod (name, namespace, uid, phase)
VALUES (:name, :namespace, :uid, :phase)
ON DUPLICATE KEY UPDATE name = VALUES(name), namespace = VALUES(namespace), uid = VALUES(uid), phase = VALUES(phase)`
		fmt.Printf("%+v\n", pod)
		_, err = p.db.NamedExecContext(context.TODO(), stmt, pod)
		if err != nil {
			return err
		}

		k8sPod := obj.(*corev1.Pod)
		// TODO: Loop over pod containers:
		for _, container := range k8sPod.Spec.Containers {
			if err := p.syncContainerVolumes(k8sPod, container); err != nil {
				return err
			}
			if err := p.syncContainerLogs(k8sPod, container); err != nil {
				return err
			}
		}

		if err := p.syncPodVolumes(k8sPod); err != nil {
			return err
		}
	}

	return nil
}

func (p *PodSync) syncPodVolumes(pod *corev1.Pod) error {
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			podPvc := schemav1.PodPvc{
				Namespace: pod.Namespace,
				PodName:   pod.Name,
				ClaimName: volume.PersistentVolumeClaim.ClaimName,
				ReadOnly:  volume.PersistentVolumeClaim.ReadOnly,
			}
			stmt := `INSERT INTO pod_pvc (namespace, pod_name, claim_name, read_only)
VALUES (:namespace, :pod_name, :claim_name, :read_only)
ON DUPLICATE KEY UPDATE namespace = VALUES(namespace), pod_name = VALUES(pod_name), claim_name = VALUES(claim_name), read_only = VALUES(read_only)`
			_, err := p.db.NamedExecContext(context.TODO(), stmt, podPvc)
			if err != nil {
				return err
			}
		} else {
			err := p.insertPodVolume(pod, volume)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *PodSync) insertPodVolume(pod *corev1.Pod, vol corev1.Volume) error {
	t, source, err := MarshalFirstNonNilStructFieldToJSON(vol.VolumeSource)
	if err != nil {
		return err
	}

	volume := schemav1.Volumes{
		Namespace:    pod.Namespace,
		PodName:      pod.Name,
		Name:         vol.Name,
		Type:         t,
		VolumeSource: source,
	}

	stmt := `INSERT INTO volumes (namespace, pod_name, name, type, volume_source)
VALUES (:namespace, :pod_name, :name, :type, :volume_source)
ON DUPLICATE KEY UPDATE namespace = VALUES(namespace), pod_name = VALUES(pod_name), name = VALUES(name), type = VALUES(type), volume_source = VALUES(volume_source)`
	_, err = p.db.NamedExecContext(context.TODO(), stmt, volume)
	if err != nil {
		return err
	}

	return nil
}

func (p *PodSync) syncContainerLogs(pod *corev1.Pod, container corev1.Container) error {
	req := p.clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: container.Name})
	body, err := req.Stream(context.TODO())
	if err != nil {
		return err
	}
	defer body.Close()
	logs, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	containerLog := schemav1.ContainerLog{
		ContainerName: container.Name,
		PodName:       pod.Name,
		Namespace:     pod.Namespace,
		Logs:          string(logs),
	}
	// TODO: Update logs in database via INSERT INTO ... ON DUPLICATE KEY. Add table for logs, i.e. container_logs.
	stmt := `INSERT INTO container_logs (namespace, pod_name, container_name, logs)
VALUES (:namespace, :pod_name, :container_name, :logs)
ON DUPLICATE KEY UPDATE namespace = VALUES(namespace), pod_name = VALUES(pod_name), 
                        container_name = VALUES(container_name), logs = VALUES(logs)`
	_, err = p.db.NamedExecContext(context.TODO(), stmt, containerLog)
	if err != nil {
		return err
	}

	return nil
}

func (p *PodSync) syncPodMetrics(pod *schemav1.Pod, containerMetrics v1beta1.ContainerMetrics) error {
	metrics, err := p.metricsClientset.MetricsV1beta1().PodMetricses(pod.Namespace).Get(context.TODO(), pod.Name,
		metav1.GetOptions{})
	if err != nil {
		return err
	}
	cpuUsage, memoryUsage, storageUsage, ephemeralStorageUsage, err := p.GetPodMetrics(pod)
	if err != nil {
		return err
	}

	podMetrics := schemav1.PodMetrics{
		Namespace:             pod.Namespace,
		PodName:               pod.Name,
		ContainerName:         containerMetrics.Name,
		Timestamp:             types.UnixMilli(metrics.Timestamp.Time),
		Duration:              metrics.Window.Duration,
		CPUUsage:              cpuUsage,
		MemoryUsage:           memoryUsage,
		StorageUsage:          storageUsage,
		EphemeralStorageUsage: ephemeralStorageUsage,
	}

	stmt := database.BuildUpsertStmt(podMetrics)
	_, err = p.db.NamedExecContext(context.TODO(), stmt, podMetrics)
	if err != nil {
		return err
	}

	return nil
}

func (p *PodSync) syncPodPersistentVolumeClaim(pod *corev1.Pod, vol corev1.Volume) error {
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			podPvc := schemav1.PodPvc{
				Namespace: pod.Namespace,
				PodName:   pod.Name,
				ClaimName: volume.PersistentVolumeClaim.ClaimName,
				ReadOnly:  volume.PersistentVolumeClaim.ReadOnly,
			}
			stmt := `INSERT INTO pod_pvc (namespace, pod_name, claim_name, read_only)
VALUES (:namespace, :pod_name, :claim_name, :read_only)
ON DUPLICATE KEY UPDATE namespace = VALUES(namespace), pod_name = VALUES(pod_name), claim_name = VALUES(claim_name), read_only = VALUES(read_only)`
			_, err := p.db.NamedExecContext(context.TODO(), stmt, podPvc)
			if err != nil {
				return err
			}
		} else {
			err := p.syncPodVolumes(pod)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *PodSync) syncContainerVolumes(pod *corev1.Pod, container corev1.Container) error {
	for _, mount := range container.VolumeMounts {
		containerVolumeMount := schemav1.ContainerVolumeMount{
			Namespace: pod.Namespace,
			PodName:   pod.Name,
			MountName: mount.Name,
			ReadOnly:  mount.ReadOnly,
			MountPath: mount.MountPath,
			SubPath:   mount.SubPath,
		}
		stmt := `INSERT INTO container_volume_mount (namespace, pod_name, mount_name, read_only, mount_path, sub_path)
VALUES (:namespace, :pod_name, :mount_name, :read_only, :mount_path, :sub_path)
ON DUPLICATE KEY UPDATE namespace = VALUES(namespace), pod_name = VALUES(pod_name), mount_name = VALUES(mount_name),
                        read_only = VALUES(read_only), mount_path = VALUES(mount_path), sub_path = VALUES(sub_path)`
		_, err := p.db.NamedExecContext(context.TODO(), stmt, containerVolumeMount)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *PodSync) WarmUp(indexer cache.Indexer) {
	stmt, err := p.db.Queryx(`SELECT namespace, name FROM pod`)
	if err != nil {
		klog.Fatal(err)
	}
	defer stmt.Close()

	for stmt.Next() {
		var pod corev1.Pod
		err := stmt.StructScan(&pod)
		if err != nil {
			log.Fatal(err)
		}
		indexer.Add(metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		})
	}
}

func MarshalFirstNonNilStructFieldToJSON(i any) (string, string, error) {
	v := reflect.ValueOf(i)
	for _, field := range reflect.VisibleFields(v.Type()) {
		if v.FieldByIndex(field.Index).IsNil() {
			continue
		}
		jsn, err := types.MarshalJSON(v.FieldByIndex(field.Index).Interface())
		if err != nil {
			return "", "", err
		}

		return field.Name, string(jsn), nil
	}

	return "", "", nil
}
