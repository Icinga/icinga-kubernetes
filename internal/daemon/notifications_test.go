package daemon

import (
	"testing"

	"github.com/icinga/icinga-go-library/notifications/source"
	"github.com/icinga/icinga-kubernetes/internal/schema"
)

func TestMergeNotificationsConfig(t *testing.T) {
	for _, test := range []struct {
		name        string
		url         string
		localUser   string
		localPass   string
		wantUser    string
		wantPass    string
		wantInvalid bool
	}{
		{name: "unix ignores stale unlocked credentials", url: "unix:///run/notifications.sock"},
		{name: "http uses unlocked credentials", url: "http://localhost:5680", wantUser: "old-http-user", wantPass: "old-http-password"},
		{name: "https uses unlocked credentials", url: "https://localhost:5680", wantUser: "old-http-user", wantPass: "old-http-password"},
		{name: "unix preserves local credential lock", url: "unix:///run/notifications.sock", localUser: "local", wantUser: "local", wantInvalid: true},
		{name: "http preserves local credentials", url: "http://localhost:5680", localUser: "local", localPass: "secret", wantUser: "local", wantPass: "secret"},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := source.Config{Username: test.localUser, Password: test.localPass}
			// Credentials precede the URL to exercise unordered database results.
			mergeNotificationsConfig(&config, []schema.Config{
				{Key: schema.ConfigKeyNotificationsUsername, Value: "old-http-user"},
				{Key: schema.ConfigKeyNotificationsPassword, Value: "old-http-password"},
				{Key: schema.ConfigKeyNotificationsUrl, Value: test.url},
				{Key: schema.ConfigKeyNotificationsIcingaWeb2Url, Value: "https://example.com/icingaweb2"},
			})
			if config.Url != test.url || config.Username != test.wantUser || config.Password != test.wantPass || config.Icingaweb2Url != "https://example.com/icingaweb2" {
				t.Fatalf("unexpected merged config for %s", test.name)
			}
			if err := config.Validate(); (err != nil) != test.wantInvalid {
				t.Fatalf("validation error = %v, want invalid %v", err, test.wantInvalid)
			}
		})
	}
}
