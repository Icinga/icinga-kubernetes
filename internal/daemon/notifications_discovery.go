package daemon

import (
	"database/sql"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/icinga/icinga-go-library/logging"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sys/unix"
)

// NotificationsDiscovery probes local Notifications prerequisites for instance heartbeats.
// Calls to Probe must be serialized.
type NotificationsDiscovery struct {
	logger          *logging.Logger
	currentUser     func() (*user.User, error)
	lastSocketError string
	lastUserError   string
}

// NewNotificationsDiscovery creates a probe using the process environment and OS identity.
func NewNotificationsDiscovery(logger *logging.Logger) *NotificationsDiscovery {
	return &NotificationsDiscovery{
		logger: logger,
		currentUser: func() (*user.User, error) {
			// LookupId retries failures that user.Current caches for the process lifetime.
			return user.LookupId(strconv.Itoa(os.Geteuid()))
		},
	}
}

// Probe returns the usable listener socket path and the daemon's OS username.
// Unavailable values are NULL. Every call rechecks availability so later heartbeats
// can discover newly created sockets and withdraw sockets that become inaccessible.
func (d *NotificationsDiscovery) Probe() (socketPath, serviceUser sql.NullString) {
	path := os.Getenv("ICINGA_NOTIFICATIONS_LISTENER_SOCKET")
	if path == "" {
		d.logger.Debug("Cannot probe Icinga Notifications listener socket availability: ICINGA_NOTIFICATIONS_LISTENER_SOCKET is not set")
		d.lastSocketError = ""
	} else {
		path = filepath.Clean(path)
		err := checkNotificationsSocket(path)
		if err != nil {
			level := zapcore.WarnLevel
			if d.lastSocketError == err.Error() {
				level = zapcore.DebugLevel
			}
			d.logger.Logw(level, "Icinga Notifications listener socket is unavailable", "path", path, "error", err)
			d.lastSocketError = err.Error()
		} else {
			if d.lastSocketError != "" {
				d.logger.Infow("Icinga Notifications listener socket is available", "path", path)
			} else {
				d.logger.Debugw("Icinga Notifications listener socket is available", "path", path)
			}
			d.lastSocketError = ""
			socketPath = sql.NullString{String: path, Valid: true}
		}
	}

	currentUser, err := d.currentUser()
	if err != nil || currentUser == nil || currentUser.Username == "" {
		message := "OS user lookup returned no username"
		if err != nil {
			message = err.Error()
		}
		level := zapcore.WarnLevel
		if d.lastUserError == message {
			level = zapcore.DebugLevel
		}
		d.logger.Logw(level, "Cannot discover Kubernetes service user", "error", message)
		d.lastUserError = message
		return socketPath, serviceUser
	}

	if d.lastUserError != "" {
		d.logger.Info("Discovered Kubernetes service user")
	}
	d.lastUserError = ""
	serviceUser = sql.NullString{String: currentUser.Username, Valid: true}
	return socketPath, serviceUser
}

// checkNotificationsSocket checks filesystem type and write access without connecting.
// It does not authenticate or send requests to the Notifications listener.
func checkNotificationsSocket(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("listener socket path must be absolute")
	}
	// The daemon administrator supplies this path through the process environment.
	info, err := os.Stat(path) // #nosec G703 -- Administrator-controlled socket path.
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return &os.PathError{Op: "probe socket", Path: path, Err: unix.ENOTSOCK}
	}
	if err := unix.Access(path, unix.W_OK); err != nil {
		return &os.PathError{Op: "access socket", Path: path, Err: err}
	}
	return nil
}
