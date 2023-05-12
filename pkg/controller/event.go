package controller

import (
	"context"
	"fmt"

	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/jmoiron/sqlx"
	eventv1 "k8s.io/api/events/v1"
	"k8s.io/client-go/tools/cache"
)

type EventSync struct {
	db *sqlx.DB
}

func NewEventSync(db *sqlx.DB) EventSync {
	return EventSync{
		db: db,
	}
}

func (p *EventSync) Sync(key string, obj interface{}, exists bool) error {
	if !exists {
		return nil
	}

	fmt.Printf("Sync/Add/Update for Event %s\n", obj.(*eventv1.Event).GetName())
	event := schemav1.NewEventFromK8s(obj.(*eventv1.Event))

	stmt := `INSERT INTO event (name, namespace, uid, reporting_controller, reporting_instance, action, reason, note, type, created, reference_kind, reference)
VALUES (:name, :namespace, :uid, :reporting_controller, :reporting_instance, :action, :reason, :note, :type, :created, :reference_kind, :reference)
ON DUPLICATE KEY UPDATE namespace = VALUES(namespace), name = VALUES(name), uid = VALUES(uid), reporting_controller = VALUES(reporting_controller), reporting_instance = VALUES(reporting_instance), action = VALUES(action), reason = VALUES(reason), note = VALUES(note), type = VALUES(type), created = VALUES(created), reference_kind = VALUES(reference_kind), reference = VALUES(reference)`
	fmt.Printf("%+v\n", event)
	_, err := p.db.NamedExecContext(context.TODO(), stmt, event)
	if err != nil {
		return err
	}

	return nil
}

// Don't warm up because we're not deleting any records for now
func (p *EventSync) WarmUp(indexer cache.Indexer) {}
