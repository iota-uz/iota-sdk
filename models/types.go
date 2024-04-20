package models

import (
	"database/sql"
	"encoding/json"
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

type JsonNullString struct {
	sql.NullString
}

type JsonNullInt32 struct {
	sql.NullInt32
}

type JsonNullInt64 struct {
	sql.NullInt64
}

type JsonNullBool struct {
	sql.NullBool
}

type JsonNullFloat64 struct {
	sql.NullFloat64
}

func (v JsonNullBool) MarshalJSON() ([]byte, error) {
	if v.Valid {
		return json.Marshal(v.Bool)
	}
	return json.Marshal(nil)
}

func (v *JsonNullBool) UnmarshalJSON(data []byte) error {
	// Unmarshalling into a pointer will let us detect null
	var x *bool
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	if x != nil {
		v.Valid = true
		v.Bool = *x
	} else {
		v.Valid = false
	}
	return nil
}

func (v JsonNullFloat64) MarshalJSON() ([]byte, error) {
	if v.Valid {
		return json.Marshal(v.Float64)
	}
	return json.Marshal(nil)
}

func (v *JsonNullFloat64) UnmarshalJSON(data []byte) error {
	// Unmarshalling into a pointer will let us detect null
	var x *float64
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	if x != nil {
		v.Valid = true
		v.Float64 = *x
	} else {
		v.Valid = false
	}
	return nil
}

func (v JsonNullInt64) MarshalJSON() ([]byte, error) {
	if v.Valid {
		return json.Marshal(v.Int64)
	}
	return json.Marshal(nil)
}

func (v *JsonNullInt64) UnmarshalJSON(data []byte) error {
	// Unmarshalling into a pointer will let us detect null
	var x *int64
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	if x != nil {
		v.Valid = true
		v.Int64 = *x
	} else {
		v.Valid = false
	}
	return nil
}

func (v JsonNullInt32) MarshalJSON() ([]byte, error) {
	if v.Valid {
		return json.Marshal(v.Int32)
	}
	return json.Marshal(nil)
}

func (v *JsonNullInt32) UnmarshalJSON(data []byte) error {
	// Unmarshalling into a pointer will let us detect null
	var x *int32
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	if x != nil {
		v.Valid = true
		v.Int32 = *x
	} else {
		v.Valid = false
	}
	return nil
}

func (v JsonNullString) MarshalJSON() ([]byte, error) {
	if v.Valid {
		return json.Marshal(v.String)
	}
	return json.Marshal(nil)
}

func (v *JsonNullString) UnmarshalJSON(data []byte) error {
	// Unmarshalling into a pointer will let us detect null
	var x *string
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	if x != nil {
		v.Valid = true
		v.String = *x
	} else {
		v.Valid = false
	}
	return nil
}

type Model interface {
	Pk() interface{}
	PkField() *Field
	Table() string
}

type Association struct {
	To     Model
	Column string
	As     string
}

type Field struct {
	Name        string
	Type        DataType
	Nullable    bool
	Association *Association
}
