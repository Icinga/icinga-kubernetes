package daemon

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/types"
)

const (
	notificationsSocketEnv     = "ICINGA_NOTIFICATIONS_LISTENER_SOCKET"
	defaultNotificationsSocket = "/run/icinga-notifications/api.sock"
)

// NotificationsDiscovery discovers the local socket and service user for instance heartbeats.
// Calls to Probe must be serialized.
type NotificationsDiscovery struct {
	logger          *logging.Logger
	path            string
	configured      bool
	currentUser     func() (string, error)
	lastSocketError string
	lastUserError   string
}

// NewNotificationsDiscovery selects the socket from the process environment. If the
// variable is absent, it uses the path from the Icinga Notifications Debian package.
// An explicitly empty variable disables the probe.
func NewNotificationsDiscovery(logger *logging.Logger) *NotificationsDiscovery {
	path, configured := os.LookupEnv(notificationsSocketEnv)
	if !configured {
		path = defaultNotificationsSocket
	}

	return &NotificationsDiscovery{
		logger:      logger,
		path:        path,
		configured:  configured,
		currentUser: lookupNotificationsServiceUser,
	}
}

// Probe returns the reachable socket path and the daemon's local OS username.
// Unavailable values are NULL. A fresh probe on each heartbeat reflects sockets
// that appear or become unavailable. Icinga Notifications resolves the peer UID
// in its own account database when authenticating an actual request.
func (d *NotificationsDiscovery) Probe(ctx context.Context) (socketPath, serviceUser types.String) {
	if d.path == "" {
		return socketPath, serviceUser
	}
	if err := checkNotificationsSocket(ctx, d.path); err != nil {
		if ctx.Err() == nil && d.lastSocketError != err.Error() {
			if !d.configured && errors.Is(err, fs.ErrNotExist) {
				d.logger.Debugw("Default Icinga Notifications socket is absent", "path", d.path)
			} else {
				d.logger.Warnw("Icinga Notifications listener socket is unavailable", "path", d.path, "error", err)
			}
			d.lastSocketError = err.Error()
		}
		return socketPath, serviceUser
	}
	if d.lastSocketError != "" {
		d.logger.Infow("Icinga Notifications listener socket is reachable", "path", d.path)
	}
	d.lastSocketError = ""
	socketPath = types.MakeString(d.path)

	username, err := d.currentUser()
	if err == nil {
		switch {
		case username == "":
			err = errors.New("OS user lookup returned no username")
		case !utf8.ValidString(username) || utf8.RuneCountInString(username) > 255:
			err = errors.New("OS username cannot be stored in the instance heartbeat")
		}
	}
	if err != nil {
		if d.lastUserError != err.Error() {
			d.logger.Warnw("Cannot discover Kubernetes service user", "error", err)
			d.lastUserError = err.Error()
		}
		return socketPath, serviceUser
	}

	if d.lastUserError != "" {
		d.logger.Info("Discovered Kubernetes service user")
	}
	d.lastUserError = ""
	return socketPath, types.MakeString(username)
}

// checkNotificationsSocket checks whether the daemon can connect without sending any requests.
func checkNotificationsSocket(ctx context.Context, path string) error {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return errors.New("listener socket path must start with a single slash")
	}
	if !utf8.ValidString(path) {
		return errors.New("listener socket path must be valid UTF-8")
	}

	dialer := net.Dialer{Timeout: time.Second}
	conn, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		return err
	}
	return conn.Close()
}
