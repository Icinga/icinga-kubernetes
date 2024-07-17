package v1

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-co-op/gocron"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/com"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"golang.org/x/sync/errgroup"
	"io"
	kcorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var (
	scheduler = gocron.NewScheduler(time.UTC)

	containerLogs   = make(map[string]ContainerLog)
	containerLogsMu sync.Mutex

	deletedPodIds = make(map[string]bool)
)

const (
	MaxConcurrentJobs int = 60
	ScheduleInterval      = 5 * time.Minute
	MaxLogLength          = 1<<16 - 1

	PodInitializing   = "PodInitializing" // https://github.com/kubernetes/kubernetes/blob/v1.30.1/pkg/kubelet/kubelet_pods.go#L80
	ContainerCreating = "ContainerCreating"

	ErrImagePull        = "ErrImagePull" // https://github.com/kubernetes/kubernetes/blob/v1.30.1/pkg/kubelet/images/types.go#L27
	ErrImagePullBackOff = "ImagePullBackOff"

	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/container/sync_result.go#L37
)

type ContainerCommon struct {
	Uuid              types.UUID
	PodUuid           types.UUID
	Name              string
	Image             string
	ImagePullPolicy   ImagePullPolicy
	State             sql.NullString
	StateDetails      sql.NullString
	IcingaState       IcingaState
	IcingaStateReason string
	Devices           []ContainerDevice `db:"-"`
	Mounts            []ContainerMount  `db:"-"`
}

func (c *ContainerCommon) Obtain(podUuid types.UUID, container kcorev1.Container, status kcorev1.ContainerStatus) {
	c.Uuid = NewUUID(podUuid, container.Name)
	c.PodUuid = podUuid
	c.Name = container.Name
	c.Image = container.Image
	c.ImagePullPolicy = ImagePullPolicy(container.ImagePullPolicy)

	state, stateDetails, err := MarshalFirstNonNilStructFieldToJSON(status.State)
	if err != nil {
		panic(err)
	}

	if state != "" {
		c.State.String = state
		c.State.Valid = true
		c.StateDetails.String = stateDetails
		c.StateDetails.Valid = true
	}

	c.IcingaState, c.IcingaStateReason = GetContainerState(container, status)

	for _, device := range container.VolumeDevices {
		c.Devices = append(c.Devices, ContainerDevice{
			ContainerUuid: c.Uuid,
			PodUuid:       c.PodUuid,
			Name:          device.Name,
			Path:          device.DevicePath,
		})
	}

	for _, mount := range container.VolumeMounts {
		m := ContainerMount{
			ContainerUuid: c.Uuid,
			PodUuid:       c.PodUuid,
			VolumeName:    mount.Name,
			Path:          mount.MountPath,
			ReadOnly: types.Bool{
				Bool:  mount.ReadOnly,
				Valid: true,
			},
		}

		if mount.SubPath != "" {
			m.SubPath.String = mount.SubPath
			m.SubPath.Valid = true
		}

		c.Mounts = append(c.Mounts, m)
	}
}

func (c *ContainerCommon) Relations() []database.Relation {
	fk := database.WithForeignKey("container_uuid")

	return []database.Relation{
		database.HasMany(c.Devices, fk),
		database.HasMany(c.Mounts, fk),

		// Allow to automatically remove the logs when a container is deleted. Otherwise, we will have some dangling
		// container logs in the database if the logs aren't deleted before removing the container, since any error
		// can interrupt the deletion process of the logs when using the `on success` mechanism.
		database.HasOne(ContainerLog{}, fk),
	}
}

type ContainerResources struct {
	CpuLimits      int64
	CpuRequests    int64
	MemoryLimits   int64
	MemoryRequests int64
}

func (c *ContainerResources) Obtain(container kcorev1.Container) {
	c.CpuLimits = container.Resources.Limits.Cpu().MilliValue()
	c.CpuRequests = container.Resources.Requests.Cpu().MilliValue()
	c.MemoryLimits = container.Resources.Limits.Memory().MilliValue()
	c.MemoryRequests = container.Resources.Requests.Memory().MilliValue()
}

type ContainerRestartable struct {
	Ready        types.Bool
	Started      types.Bool
	RestartCount int32
}

func (c *ContainerRestartable) Obtain(status kcorev1.ContainerStatus) {
	var started bool
	if status.Started != nil {
		started = *status.Started
	}

	c.Ready = types.Bool{
		Bool:  status.Ready,
		Valid: true,
	}
	c.Started = types.Bool{
		Bool:  started,
		Valid: true,
	}
	c.RestartCount = status.RestartCount
}

type InitContainer struct {
	ContainerCommon
	ContainerResources
}

func NewInitContainer(podUuid types.UUID, container kcorev1.Container, status kcorev1.ContainerStatus) *InitContainer {
	c := &InitContainer{}
	c.ContainerCommon.Obtain(podUuid, container, status)
	c.ContainerResources.Obtain(container)

	return c
}

type SidecarContainer struct {
	ContainerCommon
	ContainerResources
	ContainerRestartable
}

func NewSidecarContainer(podUuid types.UUID, container kcorev1.Container, status kcorev1.ContainerStatus) *SidecarContainer {
	c := &SidecarContainer{}
	c.ContainerCommon.Obtain(podUuid, container, status)
	c.ContainerResources.Obtain(container)
	c.ContainerRestartable.Obtain(status)

	return c
}

type Container struct {
	ContainerCommon
	ContainerResources
	ContainerRestartable
}

func NewContainer(podUuid types.UUID, container kcorev1.Container, status kcorev1.ContainerStatus) *Container {
	c := &Container{}
	c.ContainerCommon.Obtain(podUuid, container, status)
	c.ContainerResources.Obtain(container)
	c.ContainerRestartable.Obtain(status)

	return c
}

type ContainerDevice struct {
	ContainerUuid types.UUID
	PodUuid       types.UUID
	Name          string
	Path          string
}

type ContainerMount struct {
	ContainerUuid types.UUID
	PodUuid       types.UUID
	VolumeName    string
	Path          string
	SubPath       sql.NullString
	ReadOnly      types.Bool
}

type ContainerLogMeta struct {
	Logs       string          `db:"logs"`
	LastUpdate types.UnixMilli `db:"last_update"`
}

type ContainerLog struct {
	PodUuid       types.UUID `db:"pod_uuid"`
	ContainerUuid types.UUID `db:"container_uuid"`
	ContainerLogMeta

	Namespace     string `db:"-"`
	PodName       string `db:"-"`
	ContainerName string `db:"-"`
}

type ContainerStateReasonAndMassage [2]string

func (c ContainerStateReasonAndMassage) String() string {
	msg := removeTrailingWhitespaceAndFullStop(c[1])

	if msg != "" {
		return c[0] + ": " + msg
	}

	return c[0]
}

// Upsert implements the database.Upserter interface.
func (cl *ContainerLog) Upsert() interface{} {
	return cl.ContainerLogMeta
}

// syncContainerLogs fetches the logs from the kubernetes API for the given container and syncs to the database.
func (cl *ContainerLog) syncContainerLogs(ctx context.Context, clientset *kubernetes.Clientset, db *database.Database) error {
	logOptions := &kcorev1.PodLogOptions{Container: cl.ContainerName}
	if !cl.LastUpdate.Time().IsZero() {
		sinceSeconds := int64(time.Since(cl.LastUpdate.Time()).Seconds())
		logOptions.SinceSeconds = &sinceSeconds
	}

	req := clientset.CoreV1().Pods(cl.Namespace).GetLogs(cl.PodName, logOptions)
	body, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()

	logs, err := io.ReadAll(body)
	if err != nil || len(logs) == 0 {
		return err
	}

	cl.LastUpdate = types.UnixMilli(time.Now())
	cl.Logs = truncate(cl.Logs+string(logs), MaxLogLength)
	entities := make(chan interface{}, 1)
	entities <- cl
	close(entities)

	return db.UpsertStreamed(ctx, entities)
}

func GetContainerState(container kcorev1.Container, status kcorev1.ContainerStatus) (IcingaState, string) {
	if status.State.Terminated != nil {
		if status.State.Terminated.ExitCode == 0 {
			return Ok, fmt.Sprintf(
				"Container %s terminated successfully at %s.", container.Name, status.State.Terminated.FinishedAt)
		}

		if status.State.Terminated.Signal != 0 {
			return Critical, fmt.Sprintf(
				"Container %s terminated with signal %d at %s. %s.",
				container.Name,
				status.State.Terminated.Signal,
				status.State.Terminated.FinishedAt,
				ContainerStateReasonAndMassage{
					status.State.Terminated.Reason,
					removeTrailingWhitespaceAndFullStop(status.State.Terminated.Message),
				})
		}

		return Critical, fmt.Sprintf(
			"Container %s terminated with non-zero exit code %d at %s. %s.",
			container.Name,
			status.State.Terminated.ExitCode,
			status.State.Terminated.FinishedAt,
			ContainerStateReasonAndMassage{
				status.State.Terminated.Reason,
				removeTrailingWhitespaceAndFullStop(status.State.Terminated.Message),
			})
	}

	if status.State.Running != nil {
		var probe string

		if status.Started == nil || !*status.Started {
			probe = "startup"
		}

		if !status.Ready {
			probe = "liveness"
		}

		if probe != "" {
			if status.LastTerminationState.Terminated != nil {
				return Warning, fmt.Sprintf(
					"Container %s is running since %s but not ready due to failing %s probes."+
						" Last terminal with non-zero exit code %d and signal %d was at %s. %s.",
					container.Name,
					status.State.Running.StartedAt,
					probe,
					status.LastTerminationState.Terminated.ExitCode,
					status.LastTerminationState.Terminated.Signal,
					status.LastTerminationState.Terminated.FinishedAt,
					ContainerStateReasonAndMassage{
						status.LastTerminationState.Terminated.Reason,
						removeTrailingWhitespaceAndFullStop(status.LastTerminationState.Terminated.Message),
					})
			}

			return Warning, fmt.Sprintf(
				"Container %s is running since %s but not ready due to failing probes.",
				container.Name, status.State.Running.StartedAt)
		}

		return Ok, fmt.Sprintf(
			"Container %s is running since %s.", container.Name, status.State.Running.StartedAt)
	}

	if status.State.Waiting != nil {
		// TODO(el): Add Kubernetes code ref.
		if status.State.Waiting.Reason == "" {
			return Pending, fmt.Sprintf("Container %s is pending as it's waiting to be started.",
				container.Name)
		}

		if status.State.Waiting.Reason == PodInitializing {
			return Pending, fmt.Sprintf("Container %s is pending as the Pod it's running on is initializing.",
				container.Name)
		}

		if status.State.Waiting.Reason == ContainerCreating {
			return Pending, fmt.Sprintf("Container %s is pending as it's still being created.", container.Name)
		}

		if status.State.Waiting.Reason == ErrImagePull {
			// Don't flap.
			if status.LastTerminationState.Terminated != nil &&
				status.LastTerminationState.Terminated.Reason == ErrImagePullBackOff {
				return Critical, fmt.Sprintf(
					"Container %s can't start. %s.",
					container.Name,
					ContainerStateReasonAndMassage{
						status.LastTerminationState.Terminated.Reason,
						removeTrailingWhitespaceAndFullStop(status.LastTerminationState.Terminated.Message),
					})
			}

			return Warning, fmt.Sprintf("Container %s is waiting to start as its image can't be pulled: %s.",
				container.Name, status.State.Waiting.Message)
		}

		return Critical, fmt.Sprintf(
			"Container %s can't start. %s.",
			container.Name,
			ContainerStateReasonAndMassage{
				status.State.Waiting.Reason,
				removeTrailingWhitespaceAndFullStop(status.State.Waiting.Message),
			})
	}

	var reason string
	field, _json, err := MarshalFirstNonNilStructFieldToJSON(status.State)
	if err != nil {
		reason = err.Error()
	} else if field == "" {
		reason = "No state provided"
	} else {
		reason = fmt.Sprintf("%s: %s", field, _json)

	}

	return Unknown, fmt.Sprintf(
		"Container %s is unknown as its state could not be obtained: %s.",
		container.Name, reason)
}

// SyncContainers consumes from the `upsertPods` and `deletePods` chans concurrently and schedules a job for
// each of the containers (drawn from `upsertPods`) that periodically syncs the container logs with the database.
// When pods are deleted, their IDs are streamed through the `deletePods` chan, and this fetches all the container
// IDs matching the respective pod ID from the database and initiates a container deletion stream that cleans up all
// container-related resources.
func SyncContainers(ctx context.Context, db *database.Database, g *errgroup.Group, upsertPods <-chan interface{}, deletePods <-chan interface{}) {
	type containerFingerprint struct {
		Uuid    types.UUID
		PodUuid types.UUID
	}

	// Fetch all container logs from the database
	err := make(chan error, 1)
	err <- warmup(ctx, db)
	close(err)
	com.ErrgroupReceive(ctx, g, err)

	// Use buffered channel here not to block the goroutines, as they can stream container ids
	// from multiple pods concurrently.
	containerIds := make(chan interface{}, db.Options.MaxPlaceholdersPerStatement)
	g.Go(func() error {
		defer runtime.HandleCrash()

		return db.DeleteStreamed(ctx, &Container{}, containerIds, database.WithCascading())
	})

	g.Go(func() error {
		defer runtime.HandleCrash()
		defer close(containerIds)

		scheduler.SetMaxConcurrentJobs(MaxConcurrentJobs, gocron.WaitMode)
		scheduler.TagsUnique()

		scheduler.StartAsync()
		defer scheduler.Stop()

		query := db.BuildSelectStmt(&Container{}, containerFingerprint{}) + ` WHERE pod_uuid=:pod_uuid`

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case podUuid, ok := <-deletePods:
				if !ok {
					return nil
				}

				meta := &containerFingerprint{PodUuid: podUuid.(types.UUID)}
				if _, ok := deletedPodIds[meta.PodUuid.String()]; ok {
					// Due to the recursive relation resolution in the `DB#DeleteStreamed()` method, we may get the
					// same pod ID multiple times since they all share the same `on success` handler.
					break
				}
				deletedPodIds[meta.PodUuid.String()] = true

				entities, errs := db.YieldAll(ctx, func() (interface{}, error) {
					return &Container{}, nil
				}, query, meta)
				com.ErrgroupReceive(ctx, g, errs)

				g.Go(func() error {
					defer runtime.HandleCrash()

					for {
						select {
						case <-ctx.Done():
							return ctx.Err()
						case e, ok := <-entities:
							if !ok {
								return nil
							}

							container := e.(*Container)
							select {
							case containerIds <- container.Uuid:
							case <-ctx.Done():
								return ctx.Err()
							}

							err := scheduler.RemoveByTag(container.Uuid.String())
							if err != nil && !errors.Is(err, gocron.ErrJobNotFoundWithTag) {
								return err
							}

							containerLogsMu.Lock()
							delete(containerLogs, container.Uuid.String())
							containerLogsMu.Unlock()
						}
					}
				})
			case e, ok := <-upsertPods:
				if !ok {
					return nil
				}

				pod := e.(*Pod)

				delete(deletedPodIds, pod.Uuid.String())

				for _, container := range pod.Containers {
					_, err := scheduler.FindJobsByTag(container.Uuid.String())
					if err != nil && !errors.Is(err, gocron.ErrJobNotFoundWithTag) {
						return err
					}

					if container.Started.Bool && err != nil {
						containerLog := &ContainerLog{
							ContainerUuid: container.Uuid,
							PodUuid:       container.PodUuid,
							ContainerName: container.Name,
							Namespace:     pod.Namespace,
							PodName:       pod.Name,
						}

						containerLogsMu.Lock()
						if cl, ok := containerLogs[container.Uuid.String()]; ok {
							containerLog.Logs = truncate(cl.Logs, MaxLogLength)
						}
						containerLogsMu.Unlock()

						scheduler.Every(ScheduleInterval.String()).Tag(container.Uuid.String())
						_, err = scheduler.Do(containerLog.syncContainerLogs, ctx, pod.factory.clientset, db)
						if err != nil {
							return err
						}
					} else if err == nil {
						err := scheduler.RemoveByTag(container.Uuid.String())
						if err != nil {
							return err
						}

						containerLogsMu.Lock()
						delete(containerLogs, container.Uuid.String())
						containerLogsMu.Unlock()
					}
				}
			}
		}
	})
}

// warmup fetches all container logs from the database and caches them in the containerlogs variable.
func warmup(ctx context.Context, db *database.Database) error {
	g, ctx := errgroup.WithContext(ctx)

	entities, errs := db.YieldAll(ctx, func() (interface{}, error) {
		return &ContainerLog{}, nil
	}, db.BuildSelectStmt(ContainerLog{}, ContainerLog{}))
	com.ErrgroupReceive(ctx, g, errs)

	g.Go(func() error {
		defer runtime.HandleCrash()

		containerLogsMu.Lock()
		defer containerLogsMu.Unlock()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case e, ok := <-entities:
				if !ok {
					return nil
				}

				containerLog := e.(*ContainerLog)
				containerLogs[containerLog.ContainerUuid.String()] = *containerLog
			}
		}
	})

	return g.Wait()
}

// truncate truncates a UTF-8 string from the front to ensure it does not exceed the given byte length.
// It also removes content before the first newline character if one is found in the truncated string.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}

	i := len(s) - n

	// Avoid splitting a UTF-8 character.
	for i < len(s) && !utf8.RuneStart(s[i]) {
		i++
	}

	truncated := s[i:]
	if newline := strings.IndexByte(truncated, '\n'); newline != -1 {
		// Remove content before the newline and the newline character itself.
		truncated = truncated[newline+1:]
	}

	return truncated
}

// Assert that the Container type satisfies the interface compliance.
var (
	_ database.HasRelations = (*ContainerCommon)(nil)
)
