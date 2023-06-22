package database

// TableNamer implements the TableName method,
// which returns the table of the object.
type TableNamer interface {
	TableName() string // TableName tells the table.
}

type Upserter interface {
	Upsert() interface{}
}
