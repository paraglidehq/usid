package usid

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
)

// NullID can be used with the standard sql package to represent an
// ID value that can be NULL in the database.
type NullID struct {
	ID    ID
	Valid bool
}

// Compile-time interface checks for NullID
var (
	_ driver.Valuer            = NullID{}
	_ sql.Scanner              = (*NullID)(nil)
	_ json.Marshaler           = NullID{}
	_ json.Unmarshaler         = (*NullID)(nil)
	_ encoding.TextMarshaler   = NullID{}
	_ encoding.TextUnmarshaler = (*NullID)(nil)
)

// Value implements the driver.Valuer interface.
func (n NullID) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.ID.Value()
}

// Scan implements the sql.Scanner interface.
func (n *NullID) Scan(src interface{}) error {
	if src == nil {
		n.ID, n.Valid = Nil, false
		return nil
	}

	n.Valid = true
	return n.ID.Scan(src)
}

var nullJSON = []byte("null")

// MarshalJSON marshals the NullID as null or the nested ID as a string.
func (n NullID) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return nullJSON, nil
	}
	return n.ID.MarshalJSON()
}

// UnmarshalJSON unmarshals a NullID.
func (n *NullID) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		n.ID, n.Valid = Nil, false
		return nil
	}
	err := n.ID.UnmarshalJSON(b)
	n.Valid = (err == nil)
	return err
}

// MarshalText implements encoding.TextMarshaler.
func (n NullID) MarshalText() ([]byte, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.ID.MarshalText()
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (n *NullID) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		n.ID, n.Valid = Nil, false
		return nil
	}
	err := n.ID.UnmarshalText(b)
	n.Valid = (err == nil)
	return err
}
