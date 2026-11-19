package daemon

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/retry"
	k8schema "github.com/icinga/icinga-kubernetes/schema"
	"github.com/jmoiron/sqlx"
	"golang.org/x/mod/semver"
)

const (
	// The expected schema versions for MySQL and PostgreSQL.
	expectedMySQLSchemaVersion    = "v0.5.0"
	expectedPostgresSchemaVersion = "TODO: add version"
)

// ApplySchema applies any pending database migrations to the given database.
//
// If the database is empty, it will just populate it with the entire "schema.sql"
// file for the given DB driver. Otherwise, it will apply any pending migrations
// from the respective "upgrades" directory of the given DB driver.
func ApplySchema(ctx context.Context, db *database.DB) error {
	var expectedSchemaVersion, driver string
	var schema embed.FS
	switch db.DriverName() {
	case database.MySQL:
		expectedSchemaVersion = expectedMySQLSchemaVersion
		schema = k8schema.MySQLSchemaDir
		driver = "mysql"
	case database.PostgreSQL:
		expectedSchemaVersion = expectedPostgresSchemaVersion
		driver = "pgsql"
		// TODO: add PostgreSQL embedded schema directory when it is available.
	default:
		return fmt.Errorf("unsupported database driver %q", db.DriverName())
	}

	if hasSchemaTable, err := db.HasTable(ctx, "kubernetes_schema"); err != nil {
		return fmt.Errorf("cannot verify existence of database schema table: %w", err)
	} else if !hasSchemaTable {
		return applySchemaFile(ctx, db, schema, filepath.Join(driver, "schema.sql"))
	}

	haveSchemaVersion, err := checkSchemaVersion(ctx, db, expectedSchemaVersion)
	if err != nil {
		return err
	}

	if haveSchemaVersion == "" {
		return nil // The schema is up to date, nothing to do.
	}

	return applyMigrations(ctx, db, schema, driver, haveSchemaVersion, expectedSchemaVersion)
}

// applyMigrations applies any pending migrations from the given upgradesDir.
func applyMigrations(ctx context.Context, db *database.DB, schema embed.FS, driver, haveSchemaVersion, expectedSchemaVersion string) error {
	entries, err := schema.ReadDir(filepath.Join(driver, "upgrades"))
	if err != nil {
		return fmt.Errorf("cannot read upgrades directory: %w", err)
	}

	var pending []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}

		version := "v" + strings.TrimSuffix(entry.Name(), ".sql")
		if !semver.IsValid(version) {
			return fmt.Errorf("migration file %q is not named after a valid semantic version", entry.Name())
		}

		if semver.Compare(version, haveSchemaVersion) > 0 && semver.Compare(version, expectedSchemaVersion) <= 0 {
			pending = append(pending, version)
		}
	}

	// Sort the pending migrations semver-wise to ensure they are applied in the correct order.
	slices.SortFunc(pending, semver.Compare)

	// Bail out before touching the database if the migrations
	// cannot bring the schema to the expected version.
	if len(pending) == 0 || semver.Compare(pending[len(pending)-1], expectedSchemaVersion) != 0 {
		return fmt.Errorf(
			"database schema version %s is older than expected %s, but no upgrade path to %s was found in the embedded schema files",
			haveSchemaVersion, expectedSchemaVersion, expectedSchemaVersion,
		)
	}

	for _, version := range pending {
		file := path.Join(driver, "upgrades", strings.TrimLeft(version, "v")+".sql")
		if err := applySchemaFile(ctx, db, schema, file); err != nil {
			return fmt.Errorf("cannot apply migration %q: %w", file, err)
		}
	}

	// Check one last time if the schema version is now up to date after applying all pending migrations.
	if _, err := checkSchemaVersion(ctx, db, expectedSchemaVersion); err != nil {
		return fmt.Errorf(
			"after applying all pending migrations, the database schema version is still not up to date: %w",
			err,
		)
	}

	return nil
}

// applySchemaFile executes the SQL statements in the given schemaFile against the database.
func applySchemaFile(ctx context.Context, db *database.DB, dir embed.FS, file string) error {
	schema, err := dir.ReadFile(file)
	if err != nil {
		return fmt.Errorf("cannot read schema file %q: %w", file, err)
	}

	switch db.DriverName() {
	case database.PostgreSQL:
		return db.ExecTx(ctx, nil, func(ctx context.Context, tx *sqlx.Tx) error {
			if _, err := tx.ExecContext(ctx, string(schema)); err != nil {
				return fmt.Errorf("cannot execute schema file %q: %w", file, err)
			}

			return nil
		})
	default:
		// In MySQL/MariaDB, DDL statements commit implicitly, so we can't run
		// the schema import in a single transaction.
		for _, query := range database.MysqlSplitStatements(string(schema)) {
			if _, err := db.ExecContext(ctx, query); err != nil {
				return fmt.Errorf("cannot execute schema query: %w", database.CantPerformQuery(err, query))
			}
		}
	}

	return nil
}

// checkSchemaVersion checks if the database schema version is compatible with
// the expected version.
//
// It returns an error if the schema version is older than expected or if it is
// newer than expected (which would require a downgrade). Otherwise, it returns
// empty string and nil error, indicating that the schema is up to date, and the
// current schema version if it is older than expected.
func checkSchemaVersion(ctx context.Context, db *database.DB, expectedSchemaVersion string) (string, error) {
	// The schema table exists, so we need to check the current schema version
	// and apply any pending migrations.
	var dbResult []string
	err := retry.WithBackoff(
		ctx,
		func(ctx context.Context) error {
			qs := `SELECT "version" FROM kubernetes_schema ORDER BY "id" DESC LIMIT 1`
			if err := db.SelectContext(ctx, &dbResult, qs); err != nil {
				return database.CantPerformQuery(err, qs)
			}
			return nil
		},
		retry.Retryable,
		backoff.DefaultBackoff,
		db.GetDefaultRetrySettings(),
	)
	if err != nil {
		return "", err
	}

	if len(dbResult) == 0 {
		return "", errors.New("'kubernetes_schema' table exists but no schema version found")
	}

	haveSchemaVersion := dbResult[0]
	if !semver.IsValid(haveSchemaVersion) {
		return "", fmt.Errorf("database schema version %q is not a valid semantic version", haveSchemaVersion)
	}

	if result := semver.Compare(haveSchemaVersion, expectedSchemaVersion); result == 0 {
		return "", nil // The schema is up to date, nothing to do.
	} else if result > 0 {
		return "", fmt.Errorf(
			"database schema version %s is newer than expected %s, cannot downgrade schema automatically",
			haveSchemaVersion, expectedSchemaVersion,
		)
	}

	return haveSchemaVersion, nil
}
