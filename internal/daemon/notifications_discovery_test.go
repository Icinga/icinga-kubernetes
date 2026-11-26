package daemon

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/icinga/icinga-go-library/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestNotificationsDiscoveryPath(t *testing.T) {
	t.Run("Debian default", func(t *testing.T) {
		t.Setenv(notificationsSocketEnv, "")
		if err := os.Unsetenv(notificationsSocketEnv); err != nil {
			t.Fatal(err)
		}
		d := NewNotificationsDiscovery(nil)
		if d.path != "/run/icinga-notifications/api.sock" {
			t.Fatalf("unexpected default socket path %q", d.path)
		}
	})

	t.Run("custom path", func(t *testing.T) {
		t.Setenv(notificationsSocketEnv, "/run/custom/api.sock")
		d := NewNotificationsDiscovery(nil)
		if d.path != "/run/custom/api.sock" {
			t.Fatalf("unexpected custom socket path %q", d.path)
		}
	})

	t.Run("explicitly disabled", func(t *testing.T) {
		t.Setenv(notificationsSocketEnv, "")
		d := NewNotificationsDiscovery(nil)
		d.currentUser = func() (string, error) {
			t.Fatal("resolved the service user with discovery disabled")
			return "", nil
		}
		if socket, serviceUser := d.Probe(t.Context()); socket.Valid || serviceUser.Valid {
			t.Fatalf("unexpected discovery result %v, %v", socket, serviceUser)
		}
	})
}

func TestNotificationsDiscoverySocket(t *testing.T) {
	path := newTestSocketPath(t)
	t.Setenv(notificationsSocketEnv, path)
	core, logs := observer.New(zap.DebugLevel)
	d := NewNotificationsDiscovery(logging.NewLogger(zap.New(core).Sugar(), time.Second))
	d.currentUser = func() (string, error) { return "icinga-kubernetes", nil }

	for range 2 {
		if socket, serviceUser := d.Probe(t.Context()); socket.Valid || serviceUser.Valid {
			t.Fatalf("published metadata for missing socket: %v, %v", socket, serviceUser)
		}
	}
	if got := logs.FilterLevelExact(zap.WarnLevel).Len(); got != 1 {
		t.Fatalf("missing socket produced %d warnings, want 1", got)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if socket, _ := d.Probe(t.Context()); socket.Valid {
		t.Fatal("published a regular file as a socket")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	listener := listenTestSocket(t, path)
	if socket, serviceUser := d.Probe(t.Context()); !socket.Valid || socket.String != path ||
		!serviceUser.Valid || serviceUser.String != "icinga-kubernetes" {
		t.Fatalf("unexpected discovery result %v, %v", socket, serviceUser)
	}
	if got := logs.FilterLevelExact(zap.InfoLevel).Len(); got != 1 {
		t.Fatalf("socket recovery produced %d info messages, want 1", got)
	}

	t.Run("permissions", func(t *testing.T) {
		if runtime.GOOS != "linux" || os.Geteuid() == 0 {
			t.Skip("requires Linux socket permission enforcement and a non-root user")
		}
		if err := os.Chmod(path, 0o400); err != nil {
			t.Fatal(err)
		}
		if socket, _ := d.Probe(t.Context()); socket.Valid {
			t.Fatal("published an inaccessible socket")
		}
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatal(err)
		}
		if socket, _ := d.Probe(t.Context()); !socket.Valid {
			t.Fatal("socket was not rediscovered after permissions recovered")
		}
	})

	listener.SetUnlinkOnClose(false)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if socket, serviceUser := d.Probe(t.Context()); socket.Valid || serviceUser.Valid {
		t.Fatalf("published metadata for closed listener: %v, %v", socket, serviceUser)
	}
}

func TestNotificationsDiscoveryRejectsUnsupportedPaths(t *testing.T) {
	path := newTestSocketPath(t)
	listenTestSocket(t, path)
	for _, test := range []struct{ name, path string }{
		{name: "relative path", path: "api.sock"},
		{name: "double leading slash", path: "/" + path},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(notificationsSocketEnv, test.path)
			d := NewNotificationsDiscovery(logging.NewLogger(zap.NewNop().Sugar(), time.Second))
			if socket, _ := d.Probe(t.Context()); socket.Valid {
				t.Fatalf("published an unsupported socket path %q", socket.String)
			}
		})
	}
}

func TestNotificationsDiscoveryPreservesSocketPath(t *testing.T) {
	dir := filepath.Dir(newTestSocketPath(t))
	parent := filepath.Join(dir, "real")
	target := filepath.Join(parent, "child")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	listenTestSocket(t, filepath.Join(parent, "api.sock"))

	path := dir + "/link/../api.sock"
	t.Setenv(notificationsSocketEnv, path)
	d := NewNotificationsDiscovery(logging.NewLogger(zap.NewNop().Sugar(), time.Second))
	if socket, _ := d.Probe(t.Context()); !socket.Valid || socket.String != path {
		t.Fatalf("socket path = %v, want %q", socket, path)
	}
}

func TestNotificationsDiscoveryIdentity(t *testing.T) {
	path := newTestSocketPath(t)
	listenTestSocket(t, path)
	t.Setenv(notificationsSocketEnv, path)
	core, logs := observer.New(zap.DebugLevel)
	d := NewNotificationsDiscovery(logging.NewLogger(zap.New(core).Sugar(), time.Second))
	for _, result := range []struct {
		name     string
		username string
		err      error
	}{
		{name: "failed", err: errors.New("no passwd entry")},
		{name: "empty"},
		{name: "too long", username: strings.Repeat("u", 256)},
	} {
		t.Run(result.name, func(t *testing.T) {
			d.currentUser = func() (string, error) { return result.username, result.err }
			for range 2 {
				if socket, serviceUser := d.Probe(t.Context()); !socket.Valid || serviceUser.Valid {
					t.Fatalf("unexpected discovery result %v, %v", socket, serviceUser)
				}
			}
		})
	}
	if got := logs.FilterLevelExact(zap.WarnLevel).Len(); got != 3 {
		t.Fatalf("identity failures produced %d warnings, want 3 distinct failures", got)
	}
	d.currentUser = func() (string, error) { return "kubernetes", nil }
	if _, serviceUser := d.Probe(t.Context()); !serviceUser.Valid || serviceUser.String != "kubernetes" {
		t.Fatalf("resolved username was not published: %v", serviceUser)
	}
	if got := logs.FilterLevelExact(zap.InfoLevel).Len(); got != 1 {
		t.Fatalf("identity recovery produced %d info messages, want 1", got)
	}
}

func newTestSocketPath(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("/tmp", "kube-socket-") //nolint:usetesting // t.TempDir can exceed the Unix socket path limit.
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "api.sock")
}

func listenTestSocket(t *testing.T, path string) *net.UnixListener {
	t.Helper()

	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return listener
}
