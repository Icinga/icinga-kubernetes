package controller

import (
	"context"
	"fmt"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/jmoiron/sqlx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"log"
)

type NodeSync struct {
	db *sqlx.DB
}

func NewNodeSync(db *sqlx.DB) *NodeSync {
	return &NodeSync{
		db: db,
	}
}

func (n *NodeSync) Sync(key string, obj interface{}, exists bool) error {
	if !exists {
		fmt.Printf("Node %s does not exist anymore\n", key)

		namespace, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return err
		}
		_, err = n.db.Exec(`DELETE FROM node WHERE namespace=? AND name=?`, namespace, name)
		if err != nil {
			return err
		}
	} else {
		node, err := schemav1.NewNodeFromK8s(obj.(*corev1.Node))
		if err != nil {
			return err
		}
		stmt := `INSERT INTO node (name, namespace, pod_cidr, unschedulable, created, ready)
VALUES (:name, :namespace, :pod_cidr, :unschedulable, :created, :ready)
ON DUPLICATE KEY UPDATE name = VALUES(name), namespace = VALUES(namespace), pod_cidr = VALUES(pod_cidr),
                        unschedulable = VALUES(unschedulable), created = VALUES(created), ready = VALUES(ready)`
		_, err = n.db.NamedExecContext(context.TODO(), stmt, node)
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *NodeSync) WarmUp(indexer cache.Indexer) {
	stmt, err := n.db.Queryx(`SELECT namespace, name from node`)
	if err != nil {
		klog.Fatal(err)
	}
	defer stmt.Close()

	for stmt.Next() {
		var node corev1.Node
		err := stmt.StructScan(&node)
		if err != nil {
			log.Fatal(err)
		}
		indexer.Add(metav1.ObjectMeta{
			Name:      node.Name,
			Namespace: node.Namespace,
		})
	}
}
