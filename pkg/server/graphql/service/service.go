package service

import (
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

type DataType int

const (
	BigInt                DataType = iota // BigInt represents a signed eight-byte integer
	BigSerial                             // BigSerial represents an autoincrementing eight-byte integer
	Bit                                   // Bit represents a fixed-length bit string
	BitVarying                            // BitVarying represents a variable-length bit string
	Boolean                               // Boolean represents a logical Boolean (true/false)
	Box                                   // Box represents a rectangular box on a plane
	Bytea                                 // Bytea represents binary data (“byte array”)
	Character                             // Character represents a fixed-length character string
	CharacterVarying                      // CharacterVarying represents a variable-length character string
	Cidr                                  // Cidr represents an IPv4 or IPv6 network address
	Circle                                // Circle represents a circle on a plane
	Date                                  // Date represents a calendar date (year, month, day)
	DoublePrecision                       // DoublePrecision represents a double precision floating-point number (8 bytes)
	Inet                                  // Inet represents an IPv4 or IPv6 host address
	Integer                               // Integer represents a signed four-byte integer
	Interval                              // Interval represents a time span
	Json                                  // Json represents textual JSON data
	Jsonb                                 // Jsonb represents binary JSON data, decomposed
	Line                                  // Line represents an infinite line on a plane
	Lseg                                  // Lseg represents a line segment on a plane
	Macaddr                               // Macaddr represents a MAC (Media Access Control) address
	Macaddr8                              // Macaddr8 represents a MAC (Media Access Control) address (EUI-64 format)
	Money                                 // Money represents a currency amount
	Numeric                               // Numeric represents an exact numeric of selectable precision
	Path                                  // Path represents a geometric path on a plane
	PgLsn                                 // PgLsn represents a PostgreSQL Log Sequence Number
	PgSnapshot                            // PgSnapshot represents a user-level transaction ID snapshot
	Point                                 // Point represents a geometric point on a plane
	Polygon                               // Polygon represents a closed geometric path on a plane
	Real                                  // Real represents a single precision floating-point number (4 bytes)
	SmallInt                              // SmallInt represents a signed two-byte integer
	SmallSerial                           // SmallSerial represents an autoincrementing two-byte integer
	Serial                                // Serial represents an autoincrementing four-byte integer
	Text                                  // Text represents a variable-length character string
	Time                                  // Time represents a time of day (no time zone)
	TimeWithTimeZone                      // TimeWithTimeZone represents a time of day, including time zone
	Timestamp                             // Timestamp represents a date and time (no time zone)
	TimestampWithTimeZone                 // TimestampWithTimeZone represents a date and time, including time zone
	Tsquery                               // Tsquery represents a text search query
	Tsvector                              // Tsvector represents a text search document
	TxidSnapshot                          // TxidSnapshot represents a user-level transaction ID snapshot (deprecated; see pg_snapshot)
	Uuid                                  // Uuid represents a universally unique identifier
	Xml                                   // Xml represents XML data
)

type Association struct {
	To     *Model
	Column string
	As     string
}

type Field struct {
	Name        string
	Type        DataType
	Nullable    bool
	Association *Association
}

type Model struct {
	Pk     *Field
	Table  string
	Fields []*Field
}

func (m *Model) Refs() []*Field {
	var refs []*Field
	for _, field := range m.Fields {
		if field.Association != nil {
			refs = append(refs, field)
		}
	}
	return refs
}

func Get(db *sqlx.DB, query *goqu.SelectDataset) (map[string]interface{}, error) {
	stmt, _, err := query.ToSQL()
	if err != nil {
		return nil, err
	}
	row := db.QueryRowx(stmt)
	data := make(map[string]interface{})
	if err := row.MapScan(data); err != nil {
		return nil, err
	}
	return nestMap(data), nil
}

func Find(db *sqlx.DB, query *goqu.SelectDataset) ([]map[string]interface{}, error) {
	stmt, _, err := query.ToSQL()
	fmt.Println(stmt)
	if err != nil {
		return nil, err
	}
	rows, err := db.Queryx(stmt)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var data []map[string]interface{}
	for rows.Next() {
		row := map[string]interface{}{}
		if err := rows.MapScan(row); err != nil {
			return data, err
		}
		data = append(data, nestMap(row))
	}
	return data, nil
}

func Count(db *sqlx.DB, query *goqu.SelectDataset) (int, error) {
	stmt, _, err := query.Select(goqu.COUNT("*")).ToSQL()
	if err != nil {
		return 0, err
	}
	var count int
	if err := db.Get(&count, stmt); err != nil {
		return 0, err
	}
	return count, nil
}

func Create(db *sqlx.DB, query *goqu.InsertDataset) (map[string]interface{}, error) {
	stmt, _, err := query.Returning("*").ToSQL()
	if err != nil {
		return nil, err
	}
	row := db.QueryRowx(stmt)
	data := make(map[string]interface{})
	if err := row.MapScan(data); err != nil {
		return nil, err
	}
	return nestMap(data), nil
}

func Patch(db *sqlx.DB, query *goqu.UpdateDataset) (map[string]interface{}, error) {
	stmt, _, err := query.Returning("*").ToSQL()
	if err != nil {
		return nil, err
	}
	row := db.QueryRowx(stmt)
	data := make(map[string]interface{})
	if err := row.MapScan(data); err != nil {
		return nil, err
	}
	return nestMap(data), nil
}

func Remove(db *sqlx.DB, query *goqu.DeleteDataset) error {
	stmt, _, err := query.ToSQL()
	if err != nil {
		return err
	}
	resp, err := db.Exec(stmt)
	if err != nil {
		return err
	}
	if rows, err := resp.RowsAffected(); err != nil || rows == 0 {
		return fmt.Errorf("no rows affected")
	}
	return nil
}
