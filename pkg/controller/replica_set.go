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

type ReplicaSetSync struct {
	db *sqlx.DB
}

func NewReplicaSetSync(db *sqlx.DB) *ReplicaSetSync {
	return &ReplicaSetSync{
		db: db,
	}
}

func (r *ReplicaSetSync) Sync(key string, obj interface{}, exists bool) error {
	if !exists {
		fmt.Printf("Replica Set %s does not exist anymore\n", key)

		namespace, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return err
		}
		_, err = r.db.Exec(`DELETE FROM replica_set WHERE namespace=? AND name=?`, namespace, name)
		if err != nil {
			return err
		}
	} else {
		replicaSet, err := schemav1.NewReplicaSetFromK8s(obj.(*appv1.ReplicaSet))
		if err != nil {
			return err
		}
		stmt := `INSERT INTO replica_set (name, namespace, uid, desired_replicas, actual_replicas, min_ready_seconds, 
                         fully_labeled_replicas, ready_replicas, available_replicas, created)
VALUES (:name, :namespace, :uid, :desired_replicas, :actual_replicas, :min_ready_seconds, 
                         :fully_labeled_replicas, :ready_replicas, :available_replicas, :created)
ON DUPLICATE KEY UPDATE name = VALUES(name), namespace = VALUES(namespace), uid = VALUES(uid), desired_replicas = VALUES(desired_replicas),
                        actual_replicas = VALUES(actual_replicas), min_ready_seconds = VALUES(min_ready_seconds), 
                        fully_labeled_replicas = VALUES(fully_labeled_replicas), ready_replicas = VALUES(ready_replicas),
                        available_replicas = VALUES(available_replicas), created = VALUES(created)`
		_, err = r.db.NamedExecContext(context.TODO(), stmt, replicaSet)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ReplicaSetSync) WarmUp(indexer cache.Indexer) {
	stmt, err := r.db.Queryx(`SELECT namespace, name from replica_set`)
	if err != nil {
		klog.Fatal(err)
	}
	defer stmt.Close()

	for stmt.Next() {
		var replicaSet appv1.ReplicaSet
		err := stmt.StructScan(&replicaSet)
		if err != nil {
			log.Fatal(err)
		}
		indexer.Add(metav1.ObjectMeta{
			Name:      replicaSet.Name,
			Namespace: replicaSet.Namespace,
		})
	}
}
