package controller

import (
	"context"
	"fmt"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/jmoiron/sqlx"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"log"
)

type DaemonSetSync struct {
	db *sqlx.DB
}

func NewDaemonSetSync(db *sqlx.DB) *DaemonSetSync {
	return &DaemonSetSync{
		db: db,
	}
}

func (d *DaemonSetSync) Sync(key string, obj interface{}, exists bool) error {
	if !exists {
		fmt.Printf("Daemon Set %s does not exist anymore\n", key)

		namespace, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return err
		}
		_, err = d.db.Exec(`DELETE FROM daemon_set WHERE namespace=? AND name=?`, namespace, name)
		if err != nil {
			return err
		}
	} else {
		daemonSet, err := schemav1.NewDaemonSetFromK8s(obj.(*appv1.DaemonSet))
		if err != nil {
			return err
		}
		stmt := `INSERT INTO daemon_set (name, namespace, uid, min_ready_seconds, current_number_scheduled, number_misscheduled, 
                        desired_number_scheduled, number_ready, collision_count, created)
VALUES (:name, :namespace, :uid, :min_ready_seconds, :current_number_scheduled, :number_misscheduled, 
        :desired_number_scheduled, :number_ready, :collision_count, :created)
ON DUPLICATE KEY UPDATE name = VALUES(name), namespace = VALUES(namespace), uid = VALUES(uid), min_ready_seconds = VALUES(min_ready_seconds), 
                        current_number_scheduled = VALUES(current_number_scheduled), number_misscheduled = VALUES(number_misscheduled), 
                        desired_number_scheduled = VALUES(desired_number_scheduled), number_ready = VALUES(number_ready),
                        collision_count = VALUES(collision_count), created = VALUES(created)`
		_, err = d.db.NamedExecContext(context.TODO(), stmt, daemonSet)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *DaemonSetSync) WarmUp(indexer cache.Indexer) {
	stmt, err := d.db.Queryx(`SELECT namespace, name from daemon_set`)
	if err != nil {
		klog.Fatal(err)
	}
	defer stmt.Close()

	for stmt.Next() {
		var daemonSet appv1.DaemonSet
		err := stmt.StructScan(&daemonSet)
		if err != nil {
			log.Fatal(err)
		}
		indexer.Add(metav1.ObjectMeta{
			Name:      daemonSet.Name,
			Namespace: daemonSet.Namespace,
		})
	}
}
