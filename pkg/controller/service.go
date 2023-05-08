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

type ServiceSync struct {
	db *sqlx.DB
}

func NewServiceSync(db *sqlx.DB) *ServiceSync {
	return &ServiceSync{
		db: db,
	}
}

func (s *ServiceSync) Sync(key string, obj interface{}, exists bool) error {
	if !exists {
		fmt.Printf("Service %s does not exist anymore\n", key)

		namespace, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return err
		}
		_, err = s.db.Exec(`DELETE FROM service WHERE namespace=? AND name=?`, namespace, name)
		if err != nil {
			return err
		}
	} else {
		service, err := schemav1.NewServiceFromK8s(obj.(*corev1.Service))
		if err != nil {
			return err
		}
		stmt := `INSERT INTO service (name, namespace, uid, type, cluster_ip, created)
VALUES (:name, :namespace, :uid, :type, :cluster_ip, :created)
ON DUPLICATE KEY UPDATE name = VALUES(name), namespace = VALUES(namespace), uid = VALUES(uid), type = VALUES(type),
                        cluster_ip = VALUES(cluster_ip), created = VALUES(created)`
		_, err = s.db.NamedExecContext(context.TODO(), stmt, service)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *ServiceSync) WarmUp(indexer cache.Indexer) {
	stmt, err := s.db.Queryx(`SELECT namespace, name from service`)
	if err != nil {
		klog.Fatal(err)
	}
	defer stmt.Close()

	for stmt.Next() {
		var service corev1.Service
		err := stmt.StructScan(&service)
		if err != nil {
			log.Fatal(err)
		}
		indexer.Add(metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: service.Namespace,
		})
	}
}
