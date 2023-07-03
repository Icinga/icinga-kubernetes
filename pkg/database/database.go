package database

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icinga-kubernetes/pkg/backoff"
	"github.com/icinga/icinga-kubernetes/pkg/com"
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/periodic"
	"github.com/icinga/icinga-kubernetes/pkg/retry"
	"github.com/icinga/icinga-kubernetes/pkg/strcase"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/reflectx"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"k8s.io/apimachinery/pkg/util/runtime"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

var registerDriversOnce sync.Once

type FactoryFunc func() (interface{}, bool, error)

// Database is a wrapper around sqlx.DB with bulk execution,
// statement building, streaming and logging capabilities.
type Database struct {
	*sqlx.DB

	Options Options

	log logr.Logger

	columnMap *ColumnMap

	tableSemaphores   map[string]*semaphore.Weighted
	tableSemaphoresMu sync.Mutex

	quoter *Quoter
}

// NewFromConfig returns a new Database connection from the given Config.
func NewFromConfig(c *Config, log logr.Logger) (*Database, error) {
	registerDriversOnce.Do(func() {
		RegisterDrivers(log)
	})

	var dsn string
	switch c.Type {
	case "mysql":
		config := mysql.NewConfig()

		config.User = c.User
		config.Passwd = c.Password

		if IsUnixAddr(c.Host) {
			config.Net = "unix"
			config.Addr = c.Host
		} else {
			config.Net = "tcp"
			port := c.Port
			if port == 0 {
				port = 3306
			}
			config.Addr = net.JoinHostPort(c.Host, strconv.Itoa(port))
		}

		config.DBName = c.Database
		config.Timeout = time.Minute
		config.Params = map[string]string{"sql_mode": "TRADITIONAL"}

		dsn = config.FormatDSN()
	case "pgsql":
		uri := &url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(c.User, c.Password),
			Path:   "/" + url.PathEscape(c.Database),
		}

		query := url.Values{
			"connect_timeout":   {"60"},
			"binary_parameters": {"yes"},

			// Host and port can alternatively be specified in the query string. lib/pq can't parse the connection URI
			// if a Unix domain socket path is specified in the host part of the URI, therefore always use the query
			// string. See also https://github.com/lib/pq/issues/796
			"host": {c.Host},
		}
		if c.Port != 0 {
			query["port"] = []string{strconv.FormatInt(int64(c.Port), 10)}
		}

		uri.RawQuery = query.Encode()
		dsn = uri.String()
	default:
		return nil, unknownDbType(c.Type)
	}

	db, err := sqlx.Open("icinga-"+c.Type, dsn)
	if err != nil {
		return nil, errors.Wrap(err, "can't open database")
	}

	db.SetMaxIdleConns(c.Options.MaxConnections / 3)
	db.SetMaxOpenConns(c.Options.MaxConnections)

	db.Mapper = reflectx.NewMapperFunc("db", func(s string) string {
		return strcase.Snake(s)
	})

	return &Database{
		DB:              db,
		log:             log,
		columnMap:       NewColumnMap(db.Mapper),
		Options:         c.Options,
		tableSemaphores: make(map[string]*semaphore.Weighted),
		quoter:          NewQuoter(db),
	}, nil
}

// BatchSizeByPlaceholders returns how often the specified number of placeholders fits
// into Options.MaxPlaceholdersPerStatement, but at least 1.
func (db *Database) BatchSizeByPlaceholders(n int) int {
	s := db.Options.MaxPlaceholdersPerStatement / n
	if s > 0 {
		return s
	}

	return 1
}

// BuildDeleteStmt returns a DELETE statement for the given struct.
func (db *Database) BuildDeleteStmt(from interface{}) string {
	var column string
	if relation, ok := from.(Relation); ok {
		column = relation.ForeignKey()
	} else {
		column = "id"
	}

	return fmt.Sprintf(
		`DELETE FROM %s WHERE %s IN (?)`,
		db.quoter.QuoteIdentifier(TableName(from)),
		column,
	)
}

// BuildSelectStmt returns a SELECT query that creates the FROM part from the given table struct
// and the column list from the specified columns struct.
func (db *Database) BuildSelectStmt(table interface{}, columns interface{}) string {
	q := fmt.Sprintf(
		"SELECT %s FROM %s",
		db.quoter.QuoteColumns(db.columnMap.Columns(columns)),
		db.quoter.QuoteIdentifier(TableName(table)),
	)

	return q
}

// BuildUpsertStmt returns an upsert statement for the given struct.
func (db *Database) BuildUpsertStmt(subject interface{}) (stmt string, placeholders int) {
	var updateColumns []string
	insertColumns := db.columnMap.Columns(subject)
	table := TableName(subject)

	if upserter, ok := subject.(Upserter); ok {
		upsert := upserter.Upsert()
		if sliceofcolumns, ok := upsert.([]interface{}); ok {
			for _, columns := range sliceofcolumns {
				updateColumns = append(updateColumns, db.columnMap.Columns(columns)...)
			}
		} else {
			updateColumns = db.columnMap.Columns(upsert)
		}
	} else {
		updateColumns = insertColumns
	}

	var clause, setFormat string
	quoted := db.quoter.QuoteIdentifier("%[1]s")
	switch db.DriverName() {
	case MySQL:
		clause = "ON DUPLICATE KEY UPDATE"
		setFormat = fmt.Sprintf("%[1]s = VALUES(%[1]s)", quoted)
	case PostgreSQL:
		clause = fmt.Sprintf("ON CONFLICT ON CONSTRAINT pk_%s DO UPDATE SET", table)
		setFormat = `"%[1]s" = EXCLUDED."%[1]s"`
	}

	set := make([]string, 0, len(updateColumns))

	for _, col := range updateColumns {
		set = append(set, fmt.Sprintf(setFormat, col))
	}

	return fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s) %s %s`,
		db.quoter.QuoteIdentifier(table),
		db.quoter.QuoteColumns(insertColumns),
		fmt.Sprintf(":%s", strings.Join(insertColumns, ", :")),
		clause,
		strings.Join(set, ", "),
	), len(insertColumns)
}

// BulkExec bulk executes queries with a single slice placeholder in the form of `IN (?)`.
// Takes in up to the number of arguments specified in count from the arg stream,
// derives and expands a query and executes it with this set of arguments until the arg stream has been processed.
// The derived queries are executed in a separate goroutine with a weighting of 1
// and can be executed concurrently to the extent allowed by the semaphore passed in sem.
// Arguments for which the query ran successfully will be passed to onSuccess.
func (db *Database) BulkExec(
	ctx context.Context, query string, count int, sem *semaphore.Weighted, arg <-chan interface{}, features ...Feature,
) error {
	g, ctx := errgroup.WithContext(ctx)

	var counter com.Counter
	defer db.periodicLog(ctx, query, &counter).Stop()

	f := NewFeatures(features...)
	var n int64
	if f.blocking {
		n = int64(db.Options.MaxConnectionsPerTable)
	} else {
		n = 1
	}

	bulk := com.Bulk(ctx, arg, count, com.NeverSplit[any])
	g.Go(func() error {
		g, ctx := errgroup.WithContext(ctx)

		for b := range bulk {
			if err := sem.Acquire(ctx, n); err != nil {
				return errors.Wrap(err, "can't acquire semaphore")
			}

			g.Go(func(b []interface{}) func() error {
				return func() error {
					defer sem.Release(n)

					return retry.WithBackoff(
						ctx,
						func(context.Context) error {
							stmt, args, err := sqlx.In(query, b)
							if err != nil {
								return errors.Wrapf(err, "can't build placeholders for %q", query)
							}

							stmt = db.Rebind(stmt)
							_, err = db.ExecContext(ctx, stmt, args...)
							if err != nil {
								return CantPerformQuery(err, query)
							}

							counter.Add(uint64(len(b)))

							return nil
						},
						IsRetryable,
						backoff.NewExponentialWithJitter(1*time.Millisecond, 1*time.Second),
						retry.Settings{},
					)
				}
			}(b))
		}

		return g.Wait()
	})

	return g.Wait()
}

func (db *Database) Connect() bool {
	db.log.Info("Connecting to database")
	if err := db.Ping(); err != nil {
		db.log.Error(errors.WithStack(err), "Can't connect to database")

		return false
	}

	return true
}

// NamedBulkExec bulk executes queries with named placeholders in a VALUES clause most likely
// in the format INSERT ... VALUES. Takes in up to the number of entities specified in count
// from the arg stream, derives and executes a new query with the VALUES clause expanded to
// this set of arguments, until the arg stream has been processed.
// The queries are executed in a separate goroutine with a weighting of 1
// and can be executed concurrently to the extent allowed by the semaphore passed in sem.
// Entities for which the query ran successfully will be passed to onSuccess.
func (db *Database) NamedBulkExec(
	ctx context.Context, query string, count int, sem *semaphore.Weighted, arg <-chan interface{},
	splitPolicyFactory com.BulkChunkSplitPolicyFactory[interface{}], features ...Feature,
) error {
	g, ctx := errgroup.WithContext(ctx)

	var counter com.Counter
	defer db.periodicLog(ctx, query, &counter).Stop()

	bulk := com.Bulk(ctx, arg, count, splitPolicyFactory)
	with := NewFeatures(features...)

	g.Go(func() error {
		defer runtime.HandleCrash()

		for {
			select {
			case b, ok := <-bulk:
				if !ok {
					return nil
				}

				if err := sem.Acquire(ctx, 1); err != nil {
					return errors.Wrap(err, "can't acquire semaphore")
				}

				g.Go(func(b []interface{}) func() error {
					return func() error {
						defer runtime.HandleCrash()
						defer sem.Release(1)

						return retry.WithBackoff(
							ctx,
							func(ctx context.Context) error {
								_, err := db.NamedExecContext(ctx, query, b)
								if err != nil {
									return CantPerformQuery(err, query)
								}

								counter.Add(uint64(len(b)))

								if with.onSuccess != nil {
									if err := with.onSuccess(ctx, b); err != nil {
										return err
									}
								}

								return nil
							},
							IsRetryable,
							backoff.NewExponentialWithJitter(1*time.Millisecond, 1*time.Second),
							retry.Settings{},
						)
					}
				}(b))
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return g.Wait()
}

func (db *Database) GetSemaphoreForTable(table string) *semaphore.Weighted {
	db.tableSemaphoresMu.Lock()
	defer db.tableSemaphoresMu.Unlock()

	if sem, ok := db.tableSemaphores[table]; ok {
		return sem
	} else {
		sem = semaphore.NewWeighted(int64(db.Options.MaxConnectionsPerTable))
		db.tableSemaphores[table] = sem

		return sem
	}
}

// DeleteStreamed bulk deletes the specified ids via BulkExec.
// The delete statement is created using BuildDeleteStmt with the passed entityType.
// Bulk size is controlled via Options.MaxPlaceholdersPerStatement and
// concurrency is controlled via Options.MaxConnectionsPerTable.
// IDs for which the query ran successfully will be passed to onSuccess.
func (db *Database) DeleteStreamed(
	ctx context.Context, from interface{}, ids <-chan interface{}, features ...Feature,
) error {
	f := NewFeatures(features...)

	if relations, ok := from.(HasRelations); ok && f.cascading {
		var g *errgroup.Group
		g, ctx = errgroup.WithContext(ctx)
		streams := make(map[string]chan interface{}, len(relations.Relations()))
		for _, relation := range relations.Relations() {
			relation := relation

			if !relation.CascadeDelete() {
				continue
			}

			ch := make(chan interface{})
			g.Go(func() error {
				defer runtime.HandleCrash()
				defer close(ch)

				return db.DeleteStreamed(ctx, relation, ch, features...)
			})
			streams[TableName(relation)] = ch
		}

		source := ids
		ids := make(chan interface{})
		dup := make(chan interface{})

		g.Go(func() error {
			defer close(ids)
			defer close(dup)

			for {
				select {
				case entity, more := <-source:
					if !more {
						return nil
					}

					select {
					case ids <- entity:
					case <-ctx.Done():
						return ctx.Err()
					}

					select {
					case dup <- entity:
					case <-ctx.Done():
						return ctx.Err()
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		g.Go(func() error {
			defer runtime.HandleCrash()

			for {
				select {
				case entity, more := <-dup:
					if !more {
						return nil
					}

					for _, ch := range streams {
						select {
						case ch <- entity:
						case <-ctx.Done():
							return ctx.Err()
						}
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		g.Go(func() error {
			defer runtime.HandleCrash()

			return db.BulkExec(
				ctx,
				db.BuildDeleteStmt(from),
				db.Options.MaxPlaceholdersPerStatement,
				db.GetSemaphoreForTable(TableName(from)),
				ids,
				features...,
			)
		})

		return g.Wait()
	}

	return db.BulkExec(
		ctx,
		db.BuildDeleteStmt(from),
		db.Options.MaxPlaceholdersPerStatement,
		db.GetSemaphoreForTable(TableName(from)),
		ids,
		features...,
	)
}

// UpsertStreamed bulk upserts the specified entities via NamedBulkExec.
// The upsert statement is created using BuildUpsertStmt with the first entity from the entities stream.
// Bulk size is controlled via Options.MaxPlaceholdersPerStatement and
// concurrency is controlled via Options.MaxConnectionsPerTable.
func (db *Database) UpsertStreamed(
	ctx context.Context, entities <-chan interface{}, features ...Feature,
) error {
	first, forward, err := com.CopyFirst(ctx, entities)
	if first == nil {
		return errors.Wrap(err, "can't copy first entity")
	}

	sem := db.GetSemaphoreForTable(TableName(first))
	stmt, placeholders := db.BuildUpsertStmt(first)
	with := NewFeatures(features...)

	if relations, ok := first.(HasRelations); ok && with.cascading {
		var g *errgroup.Group
		g, ctx = errgroup.WithContext(ctx)
		streams := make(map[string]chan interface{}, len(relations.Relations()))
		for _, relation := range relations.Relations() {
			relation := relation

			ch := make(chan interface{})
			g.Go(func() error {
				defer runtime.HandleCrash()
				defer close(ch)

				return db.UpsertStreamed(ctx, ch)
			})
			streams[TableName(relation)] = ch
		}

		source := forward
		forward := make(chan interface{})
		dup := make(chan interface{})

		g.Go(func() error {
			defer close(forward)
			defer close(dup)

			for {
				select {
				case entity, more := <-source:
					if !more {
						return nil
					}

					select {
					case forward <- entity:
					case <-ctx.Done():
						return ctx.Err()
					}

					select {
					case dup <- entity:
					case <-ctx.Done():
						return ctx.Err()
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		g.Go(func() error {
			defer runtime.HandleCrash()

			for {
				select {
				case entity, more := <-dup:
					if !more {
						return nil
					}

					for _, relation := range entity.(HasRelations).Relations() {
						relation := relation
						g.Go(func() error {
							defer runtime.HandleCrash()

							return relation.StreamInto(ctx, streams[TableName(relation)])
						})
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		g.Go(func() error {
			defer runtime.HandleCrash()

			return db.NamedBulkExec(
				ctx, stmt, db.BatchSizeByPlaceholders(placeholders), sem, forward, com.NeverSplit[any], features...)
		})

		return g.Wait()
	}

	return db.NamedBulkExec(
		ctx, stmt, db.BatchSizeByPlaceholders(placeholders), sem, forward, com.NeverSplit[any], features...)
}

// YieldAll executes the query with the supplied scope,
// scans each resulting row into an entity returned by the factory function,
// and streams them into a returned channel.
func (db *Database) YieldAll(ctx context.Context, factoryFunc FactoryFunc, query string, scope ...interface{}) (<-chan interface{}, <-chan error) {
	g, ctx := errgroup.WithContext(ctx)
	entities := make(chan interface{}, 1)

	g.Go(func() error {
		defer runtime.HandleCrash()
		defer close(entities)

		var run func(ctx context.Context, factory FactoryFunc, query string, scope ...interface{}) error

		run = func(ctx context.Context, factory FactoryFunc, query string, scope ...interface{}) error {
			g, ctx := errgroup.WithContext(ctx)
			var counter com.Counter
			defer db.periodicLog(ctx, query, &counter).Stop()

			rows, err := db.query(ctx, query, scope...)
			if err != nil {
				return CantPerformQuery(err, query)
			}
			defer func() { _ = rows.Close() }()

			for rows.Next() {
				e, selectRecursive, err := factory()
				if err != nil {
					return errors.Wrap(err, "can't create entity")
				}

				if err = rows.StructScan(e); err != nil {
					return errors.Wrapf(err, "can't store query result into a %T: %s", e, query)
				}

				select {
				case entities <- e:
					counter.Inc()
				case <-ctx.Done():
					return ctx.Err()
				}

				if relations, ok := e.(HasRelations); ok && selectRecursive {
					for _, relation := range relations.Relations() {
						relation := relation
						fingerprint, ok := relation.TypePointer().(contracts.FingerPrinter)
						if !ok || !relation.CascadeSelect() {
							continue
						}

						g.Go(func() error {
							query := db.BuildSelectStmt(relation.TypePointer(), fingerprint.Fingerprint())
							query += fmt.Sprintf(` WHERE %s=?`, db.quoter.QuoteIdentifier(relation.ForeignKey()))

							factory := func() (interface{}, bool, error) {
								return relation.TypePointer().(contracts.Entity), true, nil
							}

							return run(ctx, factory, query, e.(contracts.IDer).ID())
						})
					}
				}
			}

			return g.Wait()
		}

		return run(ctx, factoryFunc, query, scope...)
	})

	return entities, com.WaitAsync(g)
}

func (db *Database) periodicLog(ctx context.Context, query string, counter *com.Counter) periodic.Stopper {
	return periodic.Start(ctx, 10*time.Second, func(tick periodic.Tick) {
		if count := counter.Reset(); count > 0 {
			db.log.Info(fmt.Sprintf("Executed %s with %d rows", query, count))
		}
	}, periodic.OnStop(func(tick periodic.Tick) {
		db.log.Info(fmt.Sprintf("Finished executing %s with %d rows in %s", query, counter.Total(), tick.Elapsed))
	}))
}

func (db *Database) query(ctx context.Context, query string, scope ...interface{}) (rows *sqlx.Rows, err error) {
	if len(scope) == 1 && IsStruct(scope[0]) {
		rows, err = db.NamedQueryContext(ctx, query, scope[0])
	} else {
		rows, err = db.QueryxContext(ctx, query, scope...)
	}

	return
}

func IsStruct(subject interface{}) bool {
	v := reflect.ValueOf(subject)
	switch v.Kind() {
	case reflect.Ptr:
		return v.Elem().Kind() == reflect.Struct
	case reflect.Struct:
		return true
	default:
		return false
	}
}
