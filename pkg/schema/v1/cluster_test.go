package v1

import (
	"strings"
	"testing"

	"github.com/icinga/icinga-kubernetes/pkg/database"
	mysqlschema "github.com/icinga/icinga-kubernetes/schema/mysql"
)

var _ database.HasRelations = (*Cluster)(nil)

func clusterUuidTables() map[string]struct{} {
	tables := make(map[string]struct{})
	currentTable := ""

	for _, line := range strings.Split(mysqlschema.Schema, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "CREATE" && fields[1] == "TABLE" {
			currentTable = strings.Trim(fields[2], "`")
			continue
		}

		line = strings.TrimSpace(line)
		if currentTable != "" && strings.HasPrefix(line, "cluster_uuid ") {
			tables[currentTable] = struct{}{}
		}

		if strings.HasPrefix(line, ") ENGINE=") {
			currentTable = ""
		}
	}

	return tables
}

func TestClusterRelations(t *testing.T) {
	wantTables := clusterUuidTables()
	if len(wantTables) != 20 {
		t.Fatalf("schema cluster_uuid tables: got %d, want 20", len(wantTables))
	}

	relations := (&Cluster{}).Relations()
	if len(relations) != len(wantTables) {
		t.Fatalf("cluster relations: got %d, want %d", len(relations), len(wantTables))
	}

	seen := make(map[string]struct{}, len(relations))

	for _, relation := range relations {
		table := relation.TableName()

		if _, ok := wantTables[table]; !ok {
			t.Errorf("unexpected cluster relation table %q", table)
		}

		if _, ok := seen[table]; ok {
			t.Errorf("duplicate cluster relation table %q", table)
		}
		seen[table] = struct{}{}

		if got := relation.ForeignKey(); got != "cluster_uuid" {
			t.Errorf("%s foreign key: got %q, want %q", table, got, "cluster_uuid")
		}

		if !relation.CascadeDelete() {
			t.Errorf("%s unexpectedly disables cascading deletion", table)
		}

		if entity := relation.NewEntity(); entity == nil {
			t.Errorf("%s NewEntity returned nil", table)
		}
	}

	for table := range wantTables {
		if _, ok := seen[table]; !ok {
			t.Errorf("missing cluster relation for schema table %q", table)
		}
	}
}
