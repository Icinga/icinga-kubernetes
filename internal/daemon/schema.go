package daemon

import (
	"context"
	"embed"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/retry"
	k8schema "github.com/icinga/icinga-kubernetes/schema"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/mod/semver"
)

const (
	// The expected schema versions for MySQL and PostgreSQL.
	expectedMySQLSchemaVersion    = "v0.5.1"
	expectedPostgresSchemaVersion = "TODO: add version"
)

// ApplySchema applies any pending database migrations to the given database.
//
// If the database is empty, it will just populate it with the entire "schema.sql"
// file for the given DB driver. Otherwise, it will apply any pending migrations
// from the respective "upgrades" directory of the given DB driver.
func ApplySchema(ctx context.Context, db *database.DB, l *logging.Logger) error {
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
		return errors.Errorf("unsupported database driver %q", db.DriverName())
	}

	if hasSchemaTable, err := db.HasTable(ctx, "kubernetes_schema"); err != nil {
		return errors.Wrap(err, "cannot verify existence of database schema table")
	} else if !hasSchemaTable {
		l.Info("Importing initial database schema")

		return applySchemaFile(ctx, db, schema, path.Join(driver, "schema.sql"))
	}

	haveSchemaVersion, err := checkSchemaVersion(ctx, db, expectedSchemaVersion)
	if err != nil {
		return err
	}

	if haveSchemaVersion == "" {
		return nil // The schema is up to date, nothing to do.
	}

	l.Infof("Applying pending database migrations from version %s to %s", haveSchemaVersion, expectedSchemaVersion)

	return applyMigrations(ctx, db, schema, l, driver, haveSchemaVersion, expectedSchemaVersion)
}

// applyMigrations applies any pending migrations from the given upgradesDir.
func applyMigrations(
	ctx context.Context,
	db *database.DB,
	schema embed.FS,
	l *logging.Logger,
	driver, haveSchemaVersion, expectedSchemaVersion string,
) error {
	entries, err := schema.ReadDir(path.Join(driver, "upgrades"))
	if err != nil {
		return errors.Wrap(err, "cannot read upgrades directory")
	}

	var pending []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}

		version := "v" + strings.TrimSuffix(entry.Name(), ".sql")
		if !semver.IsValid(version) {
			return errors.Errorf("migration file %q is not named after a valid semantic version", entry.Name())
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
		return errors.Errorf(
			"database schema version %s is older than expected %s, but no upgrade path to %s was found in the embedded schema files",
			haveSchemaVersion, expectedSchemaVersion, expectedSchemaVersion,
		)
	}

	for _, version := range pending {
		l.Infow("Applying database migration", zap.String("version", version))

		file := path.Join(driver, "upgrades", strings.TrimLeft(version, "v")+".sql")
		if err := applySchemaFile(ctx, db, schema, file); err != nil {
			return errors.Wrapf(err, "cannot apply migration %q", file)
		}
	}

	// Check one last time if the schema version is now up to date after applying all pending migrations.
	if have, err := checkSchemaVersion(ctx, db, expectedSchemaVersion); err != nil {
		return errors.Wrap(err, "cannot verify database schema version after applying migrations")
	} else if have != "" {
		return errors.Errorf(
			"database schema version %s is still not up to date after applying migrations, expected %s",
			have, expectedSchemaVersion,
		)
	}

	return nil
}

// applySchemaFile executes the SQL statements in the given schemaFile against the database.
func applySchemaFile(ctx context.Context, db *database.DB, dir embed.FS, file string) error {
	schema, err := dir.ReadFile(file)
	if err != nil {
		return errors.Wrapf(err, "cannot read schema file %q", file)
	}

	switch db.DriverName() {
	case database.PostgreSQL:
		return db.ExecTx(ctx, nil, func(ctx context.Context, tx *sqlx.Tx) error {
			if _, err := tx.ExecContext(ctx, string(schema)); err != nil {
				return errors.Wrapf(err, "cannot execute schema file %q", file)
			}

			return nil
		})
	default:
		// In MySQL/MariaDB, DDL statements commit implicitly, so we can't run
		// the schema import in a single transaction.
		for _, query := range database.MysqlSplitStatements(string(schema)) {
			if _, err := db.ExecContext(ctx, query); err != nil {
				var mysqlErr *mysql.MySQLError
				if file == "mysql/upgrades/0.5.1.sql" && strings.HasPrefix(query, "ALTER TABLE kubernetes_instance\n") &&
					errors.As(err, &mysqlErr) && mysqlErr.Number == 1060 {
					// ER_DUP_FIELDNAME can mean a previous attempt or another daemon
					// already added the two columns without recording this upgrade yet.
					applied, checkErr := hasSocketDiscoveryColumns(ctx, db)
					if checkErr != nil {
						return errors.Wrap(checkErr, "cannot verify existing socket discovery columns")
					}
					if applied {
						continue
					}
				}
				return errors.Wrap(database.CantPerformQuery(err, query), "cannot execute schema query")
			}
		}
	}

	return nil
}

// hasSocketDiscoveryColumns checks whether the v0.5.1 ALTER TABLE completed.
func hasSocketDiscoveryColumns(ctx context.Context, db *database.DB) (bool, error) {
	const query = `SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'kubernetes_instance'
		  AND IS_NULLABLE = 'YES' AND (COLUMN_DEFAULT IS NULL OR COLUMN_DEFAULT = 'NULL')
		  AND COLLATION_NAME = 'utf8mb4_bin' AND EXTRA = ''
		  AND ((COLUMN_NAME = 'notifications_discovered_socket_path' AND COLUMN_TYPE = 'text')
		    OR (COLUMN_NAME = 'kubernetes_service_user' AND COLUMN_TYPE = 'varchar(255)'))`
	var count int
	if err := db.GetContext(ctx, &count, query); err != nil {
		return false, database.CantPerformQuery(err, query)
	}
	return count == 2, nil
}

// checkSchemaVersion checks if the database schema version is compatible with
// the expected version.
//
// It returns an error for an unsupported version, an empty string for an
// up-to-date schema, or the normalized version for a supported upgrade.
func checkSchemaVersion(ctx context.Context, db *database.DB, expectedSchemaVersion string) (string, error) {
	// The schema table exists, so we need to check the current schema version
	// and apply any pending migrations.
	var dbResult []string
	if err := retry.WithBackoff(
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
	); err != nil {
		return "", errors.WithStack(err)
	}

	if len(dbResult) == 0 {
		return "", errors.New("'kubernetes_schema' table exists but no schema version found")
	}

	// Released schemas before v0.5.0 recorded versions without the semver prefix.
	haveSchemaVersion := "v" + strings.TrimPrefix(dbResult[0], "v")
	if !semver.IsValid(haveSchemaVersion) {
		return "", errors.Errorf("database schema version %q is not a valid semantic version", dbResult[0])
	}

	// The MySQL v0.5.0 migration assumes the table layout from v0.4.0.
	const minimumMySQLSchemaVersion = "v0.4.0"
	if db.DriverName() == database.MySQL && semver.Compare(haveSchemaVersion, minimumMySQLSchemaVersion) < 0 {
		return "", errors.Errorf(
			"database schema version %s is older than the minimum supported version %s",
			haveSchemaVersion, minimumMySQLSchemaVersion,
		)
	}

	if result := semver.Compare(haveSchemaVersion, expectedSchemaVersion); result == 0 {
		return "", nil // The schema is up to date, nothing to do.
	} else if result > 0 {
		return "", errors.Errorf(
			"database schema version %s is newer than expected %s, cannot downgrade schema automatically",
			haveSchemaVersion, expectedSchemaVersion,
		)
	}

	return haveSchemaVersion, nil
}
