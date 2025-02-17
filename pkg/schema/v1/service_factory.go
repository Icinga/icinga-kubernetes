package v1

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"k8s.io/client-go/kubernetes"
)

type ServiceFactory struct {
	clientset *kubernetes.Clientset
	db        *database.Database
	ctx       context.Context
}

func NewServiceFactory(clientset *kubernetes.Clientset, db *database.Database, ctx context.Context) *ServiceFactory {
	return &ServiceFactory{
		clientset: clientset,
		db:        db,
		ctx:       ctx,
	}
}

// GetServiceUUID fetches the UUID of a service by its namespace and name
func (f *ServiceFactory) GetServiceUUID(ctx context.Context, namespace, name string) (types.UUID, error) {
	var serviceUuid types.UUID

	query := `SELECT uuid FROM service WHERE namespace = ? AND name = ?`

	err := f.db.QueryRowContext(ctx, query, namespace, name).Scan(&serviceUuid)
	if err != nil {
		if err == sql.ErrNoRows {
			return types.UUID{}, fmt.Errorf("service '%s' not found in namespace '%s'", name, namespace)
		}
		return types.UUID{}, fmt.Errorf("error fetching service UUID: %w", err)
	}

	return serviceUuid, nil
}
