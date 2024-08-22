package v1

// Config represents a single key => value pair database config entry.
type Config struct {
	Key   ConfigKey
	Value string
}

// ConfigKey represents the database config.Key enums.
type ConfigKey string

const (
	ConfigKeyNotificationsSourceID         ConfigKey = "notifications.source_id"
	ConfigKeyNotificationsUsername         ConfigKey = "notifications.username"
	ConfigKeyNotificationsPassword         ConfigKey = "notifications.password"
	ConfigKeyNotificationsUrl              ConfigKey = "notifications.url"
	ConfigKeyNotificationsKubernetesWebUrl ConfigKey = "notifications.kubernetes_web_url"
	ConfigKeyNotificationsLocked           ConfigKey = "notifications.locked"
)
