package v1

import "github.com/icinga/icinga-go-library/types"

// Config represents a single key => value pair database config entry.
type Config struct {
	ClusterUuid types.UUID
	Key         ConfigKey
	Value       string
	Locked      types.Bool
}

// ConfigKey represents the database config.Key enums.
type ConfigKey string

const (
	ConfigKeyNotificationsUsername         ConfigKey = "notifications.username"
	ConfigKeyNotificationsPassword         ConfigKey = "notifications.password"
	ConfigKeyNotificationsUrl              ConfigKey = "notifications.url"
	ConfigKeyNotificationsKubernetesWebUrl ConfigKey = "notifications.kubernetes_web_url"
	ConfigKeyPrometheusUrl                 ConfigKey = "prometheus.url"
	ConfigKeyPrometheusInsecure            ConfigKey = "prometheus.insecure"
	ConfigKeyPrometheusUsername            ConfigKey = "prometheus.username"
	ConfigKeyPrometheusPassword            ConfigKey = "prometheus.password"
)
