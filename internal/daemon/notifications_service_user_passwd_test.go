//go:build (!cgo && !darwin) || osusergo || android

package daemon

import (
	"errors"
	"os/user"
	"strings"
	"testing"
)

func TestReadPasswdUsername(t *testing.T) {
	const passwd = `root:x:0:0::/root:/bin/sh
icinga:x:1001:1001::/var/empty:/sbin/nologin
+invalid:x:1002:1001::/var/empty:/sbin/nologin
`
	t.Run("known UID", func(t *testing.T) {
		got, err := readPasswdUsername(strings.NewReader(passwd), 1001)
		if got != "icinga" || err != nil {
			t.Fatalf("username = %q, error = %v", got, err)
		}
	})
	t.Run("missing UID", func(t *testing.T) {
		t.Setenv("USER", "env-alias")
		t.Setenv("HOME", "/home/env-alias")
		got, err := readPasswdUsername(strings.NewReader(passwd), 1002)
		if got != "" || !errors.Is(err, user.UnknownUserIdError(1002)) {
			t.Fatalf("username = %q, error = %v", got, err)
		}
	})
}
