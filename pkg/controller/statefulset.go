package controller

import (
	"context"
	"fmt"
	"log"

	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/jmoiron/sqlx"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type StatefulSetSync struct {
	db *sqlx.DB
}

func NewStatefulSetSync(db *sqlx.DB) StatefulSetSync {
	return StatefulSetSync{
		db: db,
	}
}

func (n *StatefulSetSync) Sync(key string, obj interface{}, exists bool) error {
	if exists {
		statefulSet := schemav1.NewStatefulSetFromK8s(obj.(*appsv1.StatefulSet))

		stmt := `INSERT INTO stateful_set (name, namespace, uid, replicas, service_name, ready_replicas, current_replicas, updated_replicas, available_replicas, current_revision, update_revision, collision_count)
VALUES (:name, :namespace, :uid, :replicas, :service_name, :ready_replicas, :current_replicas, :updated_replicas, :available_replicas, :current_revision, :update_revision, :collision_count)
ON DUPLICATE KEY UPDATE name = VALUES(name), namespace = VALUES(namespace), uid = VALUES(uid), replicas = VALUES(replicas), service_name = VALUES(service_name), ready_replicas = VALUES(ready_replicas), current_replicas = VALUES(current_replicas), updated_replicas = VALUES(updated_replicas), available_replicas = VALUES(available_replicas), current_revision = VALUES(current_revision), update_revision = VALUES(update_revision), collision_count = VALUES(collision_count)`
		_, err := n.db.NamedExecContext(context.TODO(), stmt, statefulSet)
		if err != nil {
			return err
		}

		return nil
	}

	fmt.Printf("Stateful Set %s does not exist anymore\n", key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	_, err = n.db.Exec(`DELETE FROM stateful_set WHERE namespace=? AND name=?`, namespace, name)
	if err != nil {
		return err
	}

	return nil
}

func (n *StatefulSetSync) WarmUp(indexer cache.Indexer) {
	stmt, err := n.db.Queryx(`SELECT namespace, name from stateful_set`)
	if err != nil {
		klog.Fatal(err)
	}
	defer stmt.Close()

	for stmt.Next() {
		var statefulSet appsv1.StatefulSet
		err := stmt.StructScan(&statefulSet)
		if err != nil {
			log.Fatal(err)
		}
		indexer.Add(metav1.ObjectMeta{
			Name:      statefulSet.Name,
			Namespace: statefulSet.Namespace,
		})
	}
}
