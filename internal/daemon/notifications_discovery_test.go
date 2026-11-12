package daemon

import (
	"errors"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/icinga/icinga-go-library/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestNotificationsDiscoverySocket(t *testing.T) {
	// macOS limits Unix socket paths to 104 bytes, so avoid the long test temp path.
	dir, err := os.MkdirTemp("/tmp", "kube-socket-") //nolint:usetesting // t.TempDir paths exceed the macOS Unix socket limit.
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "listener.sock")
	t.Setenv("ICINGA_NOTIFICATIONS_LISTENER_SOCKET", "")
	core, logs := observer.New(zap.DebugLevel)
	d := NewNotificationsDiscovery(logging.NewLogger(zap.New(core).Sugar(), time.Second))
	d.currentUser = func() (*user.User, error) { return &user.User{Username: "icinga-kubernetes"}, nil }
	if socket, serviceUser := d.Probe(); socket.Valid || !serviceUser.Valid || serviceUser.String != "icinga-kubernetes" {
		t.Fatalf("unexpected discovery: %v %v", socket, serviceUser)
	}

	t.Setenv("ICINGA_NOTIFICATIONS_LISTENER_SOCKET", "relative.sock")
	if socket, _ := d.Probe(); socket.Valid {
		t.Fatal("relative socket path was published")
	}
	logs.TakeAll()
	t.Setenv("ICINGA_NOTIFICATIONS_LISTENER_SOCKET", path)
	for range 2 {
		if socket, _ := d.Probe(); socket.Valid {
			t.Fatal("missing socket was published")
		}
	}
	if got := logs.FilterLevelExact(zap.WarnLevel).Len(); got != 1 {
		t.Fatalf("missing socket produced %d warnings, want 1", got)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if socket, _ := d.Probe(); socket.Valid {
		t.Fatal("regular file was published")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	if socket, _ := d.Probe(); !socket.Valid || socket.String != path {
		t.Fatalf("usable socket was not published: %v", socket)
	}
	if got := logs.FilterLevelExact(zap.InfoLevel).Len(); got != 1 {
		t.Fatalf("socket recovery produced %d info messages, want 1", got)
	}

	t.Run("permissions", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("root bypasses socket permission checks")
		}
		if err := os.Chmod(path, 0o400); err != nil {
			t.Fatal(err)
		}
		if socket, _ := d.Probe(); socket.Valid {
			t.Fatal("unwritable socket was published")
		}
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatal(err)
		}
		if socket, _ := d.Probe(); !socket.Valid {
			t.Fatal("socket was not rediscovered after permissions recovered")
		}
	})
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if socket, _ := d.Probe(); socket.Valid {
		t.Fatal("removed socket was published")
	}
}

func TestNotificationsDiscoveryIdentity(t *testing.T) {
	t.Setenv("ICINGA_NOTIFICATIONS_LISTENER_SOCKET", "")
	core, logs := observer.New(zap.DebugLevel)
	d := NewNotificationsDiscovery(logging.NewLogger(zap.New(core).Sugar(), time.Second))
	for _, result := range []struct {
		name string
		user *user.User
		err  error
	}{
		{name: "failed", err: errors.New("no passwd entry")},
		{name: "empty", user: &user.User{}},
		{name: "nil"},
	} {
		t.Run(result.name, func(t *testing.T) {
			d.currentUser = func() (*user.User, error) { return result.user, result.err }
			for range 2 {
				if _, serviceUser := d.Probe(); serviceUser.Valid || serviceUser.String != "" {
					t.Fatalf("unresolved username was published: %v", serviceUser)
				}
			}
		})
	}
	if got := logs.FilterLevelExact(zap.WarnLevel).Len(); got != 2 {
		t.Fatalf("identity failures produced %d warnings, want 2 distinct failures", got)
	}
	d.currentUser = func() (*user.User, error) { return &user.User{Username: "kubernetes"}, nil }
	if _, serviceUser := d.Probe(); !serviceUser.Valid || serviceUser.String != "kubernetes" {
		t.Fatalf("resolved username was not published: %v", serviceUser)
	}
	if got := logs.FilterLevelExact(zap.InfoLevel).Len(); got != 1 {
		t.Fatalf("identity recovery produced %d info messages, want 1", got)
	}
}
