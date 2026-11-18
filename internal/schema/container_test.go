package schema

import (
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
	"time"

	"github.com/icinga/icinga-go-library/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// TestSyncContainerLogsForbidden checks that a namespace is warned about only
// the first time its container logs are denied, while each failure to get
// container logs is logged at debug level.
func TestSyncContainerLogsForbidden(t *testing.T) {
	// The status codes the server responds with per pod. It serves an empty log
	// for all others, which makes syncContainerLogs return before it writes to
	// the database, so that none is needed.
	status := map[string]int{"denied": http.StatusForbidden, "failed": http.StatusBadRequest}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The path ends in /pods/<name>/log.
		if code, ok := status[path.Base(path.Dir(r.URL.Path))]; ok {
			http.Error(w, http.StatusText(code), code)
		}
	}))
	defer server.Close()

	clientset, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	core, logs := observer.New(zap.DebugLevel)
	log := logging.NewLogger(zap.New(core).Sugar(), time.Second)

	const namespace = "forbidden-logs"

	// forbiddenLogNamespaces is shared, so forget what this test denied for
	// later runs.
	t.Cleanup(func() { forbiddenLogNamespaces.Delete(namespace) })

	// syncLogs syncs the logs of pod and checks how many warnings and debug
	// logs about failures there are so far.
	syncLogs := func(pod string, warned, failed int) {
		t.Helper()

		cl := &ContainerLog{Namespace: namespace, PodName: pod}
		if err := cl.syncContainerLogs(t.Context(), clientset, nil, log); (err != nil) != (status[pod] != 0) {
			t.Fatalf("syncContainerLogs(%s) = %v", pod, err)
		}

		if got := logs.FilterMessage("Cannot get container logs. Skipping until permitted").Len(); got != warned {
			t.Fatalf("warned %d times, want %d", got, warned)
		}

		if got := logs.FilterMessage("Cannot get container logs").Len(); got != failed {
			t.Fatalf("logged %d failures, want %d", got, failed)
		}
	}

	syncLogs("denied", 1, 1)
	syncLogs("permitted", 1, 1)
	syncLogs("denied", 1, 2)

	// Other errors are not about permissions.
	syncLogs("failed", 1, 3)
}
