//go:build (cgo || darwin) && !osusergo && !android

package daemon

import (
	"os"
	"os/user"
	"strconv"
)

func lookupNotificationsServiceUser() (string, error) {
	account, err := user.LookupId(strconv.Itoa(os.Geteuid()))
	if err != nil {
		return "", err
	}
	return account.Username, nil
}
