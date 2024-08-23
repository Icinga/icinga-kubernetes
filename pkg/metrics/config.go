package metrics

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/database"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/pkg/errors"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"strings"
)

// PrometheusConfig defines Prometheus configuration.
type PrometheusConfig struct {
	Url      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// Validate checks constraints in the supplied Prometheus configuration and returns an error if they are violated.
func (c *PrometheusConfig) Validate() error {
	if (c.Username == "") != (c.Password == "") {
		return errors.New("both username and password must be provided")
	}
	return nil
}

func SyncPrometheusConfig(ctx context.Context, db *database.DB, config *PrometheusConfig) error {
	var configPairs []*schemav1.Config

	if config.Url != "" {
		configPairs = append(configPairs, &schemav1.Config{Key: schemav1.ConfigKeyPrometheusUrl, Value: config.Url, Locked: "y"})

		if config.Username != "" {
			configPairs = append(configPairs, &schemav1.Config{Key: schemav1.ConfigKeyPrometheusUsername, Value: config.Username, Locked: "y"})
			configPairs = append(configPairs, &schemav1.Config{Key: schemav1.ConfigKeyPrometheusPassword, Value: "", Locked: "y"})
		} else {
			var deleteKeys []schemav1.ConfigKey

			deleteKeys = append(deleteKeys, schemav1.ConfigKeyPrometheusUsername)
			deleteKeys = append(deleteKeys, schemav1.ConfigKeyPrometheusPassword)

			deleteStmt := fmt.Sprintf(
				`DELETE FROM %s WHERE %s = (?)`,
				database.TableName(&schemav1.Config{}),
				"`key`",
			)

			for _, key := range deleteKeys {
				if _, err := db.ExecContext(ctx, deleteStmt, key); err != nil {
					return errors.Wrap(err, "cannot delete Prometheus credentials")
				}
			}
		}
	} else {
		err := RetrievePrometheusConfig(ctx, db, config)
		if err != nil {
			return err
		}

		configPairs = append(configPairs, &schemav1.Config{Key: schemav1.ConfigKeyPrometheusUrl, Value: config.Url, Locked: "n"})

		if config.Username != "" {
			configPairs = append(configPairs, &schemav1.Config{Key: schemav1.ConfigKeyPrometheusUsername, Value: config.Username, Locked: "n"})
			configPairs = append(configPairs, &schemav1.Config{Key: schemav1.ConfigKeyPrometheusPassword, Value: config.Password, Locked: "n"})
		}
	}

	stmt, _ := db.BuildUpsertStmt(&schemav1.Config{})
	if _, err := db.NamedExecContext(ctx, stmt, configPairs); err != nil {
		return errors.Wrap(err, "cannot upsert Prometheus configuration")
	}

	return nil
}

func RetrievePrometheusConfig(ctx context.Context, db *database.DB, config *PrometheusConfig) error {
	var dbConfig []*schemav1.Config
	if err := db.SelectContext(ctx, &dbConfig, db.BuildSelectStmt(&schemav1.Config{}, &schemav1.Config{})); err != nil {
		return errors.Wrap(err, "cannot retrieve Prometheus configuration")
	}

	for _, c := range dbConfig {
		switch c.Key {
		case schemav1.ConfigKeyPrometheusUrl:
			config.Url = c.Value
		case schemav1.ConfigKeyPrometheusUsername:
			config.Username = c.Value
		case schemav1.ConfigKeyPrometheusPassword:
			config.Password = c.Value
		}
	}

	return nil
}

func AutoDetectPrometheus(ctx context.Context, clientset *kubernetes.Clientset, config *PrometheusConfig) error {
	services, err := clientset.CoreV1().Services("monitoring").List(ctx, kmetav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=prometheus",
	})
	if err != nil {
		return errors.Wrap(err, "cannot list Prometheus services")
	}

	if len(services.Items) == 0 {
		return errors.New("no Prometheus service found")
	}

	var ip string
	var port int32

	// Check if we are running in a Kubernetes cluster. If so, use the
	// service's ClusterIP. Otherwise, use the API Server's IP and NodePort.
	if _, err = rest.InClusterConfig(); err == nil {
		for _, service := range services.Items {
			if service.Spec.Type == "ClusterIP" {
				ip = services.Items[0].Spec.ClusterIP
				port = services.Items[0].Spec.Ports[0].Port

				break
			}
		}
	} else if errors.Is(err, rest.ErrNotInCluster) {
		for _, service := range services.Items {
			if service.Spec.Type == "NodePort" {
				ip = strings.Split(clientset.RESTClient().Get().URL().Host, ":")[0]
				port = service.Spec.Ports[0].NodePort

				break
			}
		}
	}

	config.Url = fmt.Sprintf("http://%s:%d", ip, port)

	return nil
}
