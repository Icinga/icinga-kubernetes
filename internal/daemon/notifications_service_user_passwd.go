//go:build (!cgo && !darwin) || osusergo || android

package daemon

import (
	"bufio"
	"io"
	"os"
	"os/user"
	"strconv"
	"strings"
)

func lookupNotificationsServiceUser() (string, error) {
	// The pure Go os/user lookup can synthesize an account from USER and HOME.
	// An authenticated socket source needs an account that exists independently.
	passwd, err := os.Open("/etc/passwd")
	if err != nil {
		return "", err
	}
	defer func() { _ = passwd.Close() }()

	return readPasswdUsername(passwd, os.Geteuid())
}

func readPasswdUsername(passwd io.Reader, uid int) (string, error) {
	id := strconv.Itoa(uid)
	lines := bufio.NewScanner(passwd)
	for lines.Scan() {
		fields := strings.SplitN(strings.TrimSpace(lines.Text()), ":", 7)
		if len(fields) != 7 || fields[2] != id {
			continue
		}
		username := fields[0]
		if username != "" && strings.IndexByte("#+-", username[0]) < 0 {
			return username, nil
		}
	}
	if err := lines.Err(); err != nil {
		return "", err
	}
	return "", user.UnknownUserIdError(uid)
}
