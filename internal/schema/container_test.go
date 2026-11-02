package schema

import (
	"net/http"
	"net/http/httptest"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// logCounts counts the logs about container logs that cannot be got, per
// namespace and per pod.
type logCounts struct {
	warned, permitted, gone          int
	podDenied, podPermitted, podGone int
	failed                           int
}

// TestSyncContainerLogsForbidden checks that a namespace is warned about once
// its first pod is denied to get container logs, and logged about once its
// last denied pod is permitted, gone or finished, while each pod is logged
// about on its own. Pods permitted alongside denied ones do not log anything,
// and other errors are logged about on their own.
func TestSyncContainerLogsForbidden(t *testing.T) {
	// The status codes the server responds with per pod. It serves an empty log
	// for all others, which makes syncContainerLogs return before it writes to
	// the database, so that none is needed.
	var status sync.Map

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The path ends in /pods/<name>/log.
		if code, ok := status.Load(path.Base(path.Dir(r.URL.Path))); ok {
			http.Error(w, http.StatusText(code.(int)), code.(int))
		}
	}))
	defer server.Close()

	clientset, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	core, logs := observer.New(zap.DebugLevel)
	log := logging.NewLogger(zap.New(core).Sugar(), time.Second)

	count := func(level zapcore.Level, msg string) int {
		return logs.FilterLevelExact(level).FilterMessage(msg).Len()
	}

	// check checks how many logs about container logs that cannot be got there
	// are so far.
	check := func(want logCounts) {
		t.Helper()

		got := logCounts{
			warned:       count(zap.WarnLevel, "Cannot get container logs. Skipping denied pods until permitted"),
			permitted:    count(zap.InfoLevel, "Permitted to get container logs now. No longer skipping"),
			gone:         count(zap.InfoLevel, "Denied pods are gone or finished. No longer skipping"),
			podDenied:    count(zap.DebugLevel, "Cannot get container logs. Skipping until permitted"),
			podPermitted: count(zap.DebugLevel, "Permitted to get container logs now. No longer skipping"),
			podGone:      count(zap.DebugLevel, "Denied pod is gone or finished"),
			failed:       count(zap.DebugLevel, "Cannot get container logs"),
		}

		if got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	}

	// syncLogs syncs the logs of cl with the server responding with code, or an
	// empty log if it is 0, and checks the logs about it so far.
	syncLogs := func(cl *ContainerLog, code int, want logCounts) {
		t.Helper()

		if code == 0 {
			status.Delete(cl.PodName)
		} else {
			status.Store(cl.PodName, code)
		}

		if err := cl.syncContainerLogs(t.Context(), clientset, nil, log); (err != nil) != (code != 0) {
			t.Fatalf("syncContainerLogs(%s) = %v, want an error: %t", cl.PodName, err, code != 0)
		}

		check(want)
	}

	newContainerLog := func(pod string) *ContainerLog {
		return &ContainerLog{
			ContainerUuid: types.UUID{UUID: uuid.New()},
			PodUuid:       types.UUID{UUID: uuid.New()},
			Namespace:     "forbidden-logs",
			PodName:       pod,
			ContainerName: "container",
		}
	}

	a, b, c := newContainerLog("a"), newContainerLog("b"), newContainerLog("c")

	// denyLogs records only pods whose containers a job syncs, so each gets one.
	// The scheduler is not started, so they never run.
	for _, cl := range []*ContainerLog{a, b, c} {
		if _, err := scheduler.Every("1h").Tag(cl.ContainerUuid.String()).Do(func() {}); err != nil {
			t.Fatal(err)
		}
	}

	// deniedPods and the scheduler are shared, so remove what this test denied
	// and scheduled for later runs.
	t.Cleanup(func() {
		for _, cl := range []*ContainerLog{a, b, c} {
			_ = scheduler.RemoveByTag(cl.ContainerUuid.String())
		}

		deniedPodsMu.Lock()
		delete(deniedPods, a.Namespace)
		deniedPodsMu.Unlock()
	})

	// removePod deletes or finishes the pod of cl as SyncContainers does, which
	// removes the jobs of its containers before it forgets the pod.
	removePod := func(cl *ContainerLog, want logCounts) {
		t.Helper()

		if err := scheduler.RemoveByTag(cl.ContainerUuid.String()); err != nil {
			t.Fatal(err)
		}
		forgetLogs(log, cl.PodUuid)

		check(want)
	}

	// Only some pods are permitted, which neither warns again nor logs the
	// namespace as permitted.
	syncLogs(a, http.StatusForbidden, logCounts{warned: 1, podDenied: 1})
	syncLogs(b, 0, logCounts{warned: 1, podDenied: 1})
	syncLogs(a, http.StatusForbidden, logCounts{warned: 1, podDenied: 1})
	syncLogs(b, 0, logCounts{warned: 1, podDenied: 1})

	// The namespace is permitted only once its last denied pod is.
	syncLogs(b, http.StatusForbidden, logCounts{warned: 1, podDenied: 2})
	syncLogs(a, 0, logCounts{warned: 1, podDenied: 2, podPermitted: 1})
	syncLogs(b, 0, logCounts{warned: 1, permitted: 1, podDenied: 2, podPermitted: 2})

	// Other errors are not about permissions.
	syncLogs(a, http.StatusBadRequest, logCounts{warned: 1, permitted: 1, podDenied: 2, podPermitted: 2, failed: 1})

	// The namespace is no longer skipped once its last denied pod is gone or
	// finished, but not logged as permitted.
	syncLogs(a, http.StatusForbidden, logCounts{warned: 2, permitted: 1, podDenied: 3, podPermitted: 2, failed: 1})
	syncLogs(b, http.StatusForbidden, logCounts{warned: 2, permitted: 1, podDenied: 4, podPermitted: 2, failed: 1})
	removePod(a, logCounts{warned: 2, permitted: 1, podDenied: 4, podPermitted: 2, podGone: 1, failed: 1})
	removePod(b, logCounts{warned: 2, permitted: 1, gone: 1, podDenied: 4, podPermitted: 2, podGone: 2, failed: 1})

	// A sync still running when its pod is removed does not deny the pod again,
	// or c would not be warned about as the first denied pod.
	syncLogs(a, http.StatusForbidden, logCounts{warned: 2, permitted: 1, gone: 1, podDenied: 4, podPermitted: 2, podGone: 2, failed: 1})
	syncLogs(c, http.StatusForbidden, logCounts{warned: 3, permitted: 1, gone: 1, podDenied: 5, podPermitted: 2, podGone: 2, failed: 1})
}
