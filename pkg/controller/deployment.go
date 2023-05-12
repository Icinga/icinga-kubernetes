package controller

import (
	"context"
	"fmt"
	"log"

	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/jmoiron/sqlx"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type DeploymentSync struct {
	db *sqlx.DB
}

func NewDeploymentSync(db *sqlx.DB) DeploymentSync {
	return DeploymentSync{
		db: db,
	}
}

func (p *DeploymentSync) Sync(key string, obj interface{}, exists bool) error {
	if exists {
		fmt.Printf("Sync/Add/Update for Deployment %s\n", obj.(*appv1.Deployment).GetName())
		deployment := schemav1.NewDeploymentFromK8s(obj.(*appv1.Deployment))

		stmt := `INSERT INTO deployment (name, namespace, uid, strategy, paused, replicas, available_replicas, ready_replicas, unavailable_replicas, collision_count, created)
VALUES (:name, :namespace, :uid, :strategy, :paused, :replicas, :available_replicas, :ready_replicas, :unavailable_replicas, :collision_count, :created)
ON DUPLICATE KEY UPDATE name = VALUES(name), namespace = VALUES(namespace), uid = VALUES(uid), strategy = VALUES(strategy), paused = VALUES(paused),
						replicas = VALUES(replicas), available_replicas = VALUES(available_replicas), ready_replicas = VALUES(ready_replicas),
						unavailable_replicas = VALUES(unavailable_replicas), collision_count = VALUES(collision_count), created = VALUES(created)`
		fmt.Printf("%+v\n", deployment)
		_, err := p.db.NamedExecContext(context.TODO(), stmt, deployment)
		if err != nil {
			return err
		}

		return nil
	}

	fmt.Printf("Deployment %s does not exist anymore\n", key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	_, err = p.db.Exec(`DELETE FROM deployment WHERE namespace=? AND name=?`, namespace, name)
	if err != nil {
		return err
	}

	return nil
}

func (p *DeploymentSync) WarmUp(indexer cache.Indexer) {
	stmt, err := p.db.Queryx(`SELECT namespace, name from deployment`)
	if err != nil {
		klog.Fatal(err)
	}
	defer stmt.Close()

	for stmt.Next() {
		var deployment appv1.Deployment
		err := stmt.StructScan(&deployment)
		if err != nil {
			log.Fatal(err)
		}

		indexer.Add(metav1.ObjectMeta{
			Name:      deployment.Name,
			Namespace: deployment.Namespace,
		})
	}
}
