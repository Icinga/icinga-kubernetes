package internal

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/metrics"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"strings"
)

func SyncPrometheusConfig(ctx context.Context, db *database.DB, config *metrics.PrometheusConfig, clusterUuid types.UUID) error {
	_true := types.Bool{Bool: true, Valid: true}

	if config.Url != "" {
		toDb := []schemav1.Config{
			{ClusterUuid: clusterUuid, Key: schemav1.ConfigKeyPrometheusUrl, Value: config.Url, Locked: _true},
		}

		if config.Insecure != "" {
			toDb = append(
				toDb,
				schemav1.Config{ClusterUuid: clusterUuid, Key: schemav1.ConfigKeyPrometheusInsecure, Value: config.Insecure, Locked: _true},
			)
		}

		if config.Username != "" {
			toDb = append(
				toDb,
				schemav1.Config{ClusterUuid: clusterUuid, Key: schemav1.ConfigKeyPrometheusUsername, Value: config.Username, Locked: _true},
				schemav1.Config{ClusterUuid: clusterUuid, Key: schemav1.ConfigKeyPrometheusPassword, Value: config.Password, Locked: _true},
			)
		}

		err := db.ExecTx(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
			if _, err := tx.ExecContext(
				ctx,
				fmt.Sprintf(
					`DELETE FROM "%s" WHERE "cluster_uuid" = ? AND "key" LIKE ? AND "locked" = ?`,
					database.TableName(&schemav1.Config{}),
				),
				clusterUuid,
				`prometheus.%`,
				_true,
			); err != nil {
				return errors.Wrap(err, "cannot delete Prometheus config")
			}

			stmt, _ := db.BuildUpsertStmt(schemav1.Config{})
			if _, err := tx.NamedExecContext(ctx, stmt, toDb); err != nil {
				return errors.Wrap(err, "cannot insert Prometheus config")
			}

			return nil
		})
		if err != nil {
			return errors.Wrap(err, "cannot upsert Prometheus config")
		}
	} else {
		err := db.ExecTx(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
			if _, err := tx.ExecContext(
				ctx,
				fmt.Sprintf(
					`DELETE FROM "%s" WHERE "cluster_uuid" = ? AND "key" LIKE ? AND "locked" = ?`,
					database.TableName(&schemav1.Config{}),
				),
				clusterUuid,
				`prometheus.%`,
				_true,
			); err != nil {
				return errors.Wrap(err, "cannot delete Prometheus config")
			}

			rows, err := tx.QueryxContext(ctx, db.BuildSelectStmt(&schemav1.Config{}, &schemav1.Config{}))
			if err != nil {
				return errors.Wrap(err, "cannot fetch Prometheus config from DB")
			}

			for rows.Next() {
				var r schemav1.Config
				if err := rows.StructScan(&r); err != nil {
					return errors.Wrap(err, "cannot fetch Prometheus config from DB")
				}

				switch r.Key {
				case schemav1.ConfigKeyPrometheusUrl:
					config.Url = r.Value
				case schemav1.ConfigKeyPrometheusInsecure:
					config.Insecure = r.Value
				case schemav1.ConfigKeyPrometheusUsername:
					config.Username = r.Value
				case schemav1.ConfigKeyPrometheusPassword:
					config.Password = r.Value
				}
			}

			return nil
		})
		if err != nil {
			return errors.Wrap(err, "cannot retrieve Prometheus config")
		}
	}

	return nil
}

// AutoDetectPrometheus tries to auto-detect the Prometheus service in the monitoring namespace and
// if found sets the URL in the supplied Prometheus configuration. The first service with the label
// "app.kubernetes.io/name=prometheus" is used. Until now the ServiceTypes ClusterIP and NodePort are supported.
func AutoDetectPrometheus(ctx context.Context, clientset *kubernetes.Clientset, config *metrics.PrometheusConfig) error {
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
			if service.Spec.Type == v1.ServiceTypeClusterIP {
				ip = service.Spec.ClusterIP
				port = service.Spec.Ports[0].Port

				break
			}
		}
	} else if errors.Is(err, rest.ErrNotInCluster) {
		for _, service := range services.Items {
			if service.Spec.Type == v1.ServiceTypeNodePort {
				ip = strings.Split(clientset.RESTClient().Get().URL().Host, ":")[0]
				port = service.Spec.Ports[0].NodePort

				break
			}
		}
	}

	if ip == "" {
		return errors.New("no Prometheus found")
	}

	config.Url = fmt.Sprintf("http://%s:%d", ip, port)

	return nil
}
