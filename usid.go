// Package usid provides microsecond-precision, time-ordered unique identifiers.
//
// USIDs are 64-bit IDs with an embedded timestamp, node ID, and sequence number.
// They sort chronologically, are URL-safe when encoded, and work well as database
// primary keys.
//
// Basic usage:
//
//	usid.SetNodeID(1)  // Call once at startup
//	id := usid.New()   // Generate IDs
//	fmt.Println(id)    // Crockford Base32 encoded by default
//
// The bit layout is configurable via Epoch, NodeBits, and SeqBits variables.
package usid

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/paraglidehq/usid/v2/base58"
	"github.com/paraglidehq/usid/v2/crockford"
)

// Compile-time interface checks for ID
var (
	_ fmt.Stringer               = ID(0)
	_ driver.Valuer              = ID(0)
	_ sql.Scanner                = (*ID)(nil)
	_ encoding.TextMarshaler     = ID(0)
	_ encoding.TextUnmarshaler   = (*ID)(nil)
	_ encoding.BinaryMarshaler   = ID(0)
	_ encoding.BinaryUnmarshaler = (*ID)(nil)
	_ json.Marshaler             = ID(0)
	_ json.Unmarshaler           = (*ID)(nil)
	_ gob.GobEncoder             = ID(0)
	_ gob.GobDecoder             = (*ID)(nil)
)

// Format specifies the string encoding format for IDs.
type Format string

// Supported ID string formats.
const (
	FormatCrockford Format = "crockford" // Crockford Base32, case-insensitive (default)
	FormatBase58  Format = "base58"  // URL-safe, compact
	FormatBase64  Format = "base64"  // Standard base64 encoding
	FormatHash    Format = "hash"    // Hexadecimal encoding
	FormatDecimal Format = "decimal" // Decimal integer string
)

// ID is a 64-bit microsecond-precision time-ordered identifier.
type ID int64

// Nil is the zero ID, representing an absent or invalid ID.
var Nil ID = 0

// Omni is the maximum ID value (math.MaxInt64), useful as an upper bound in queries.
var Omni ID = math.MaxInt64

// Int64 returns the ID as an int64.
func (id ID) Int64() int64 {
	return int64(id)
}

// IsNil returns true if the ID is Nil (zero).
func (id ID) IsNil() bool {
	return id == Nil
}

// Bytes returns the ID as an 8-byte big-endian slice.
func (id ID) Bytes() []byte {
	b := make([]byte, 8)
	b[0] = byte(id >> 56)
	b[1] = byte(id >> 48)
	b[2] = byte(id >> 40)
	b[3] = byte(id >> 32)
	b[4] = byte(id >> 24)
	b[5] = byte(id >> 16)
	b[6] = byte(id >> 8)
	b[7] = byte(id)
	return b
}

// Hash returns the ID as an 8-byte big-endian array (for hex formatting).
func (id ID) Hash() [8]byte {
	return [8]byte{
		byte(id >> 56),
		byte(id >> 48),
		byte(id >> 40),
		byte(id >> 32),
		byte(id >> 24),
		byte(id >> 16),
		byte(id >> 8),
		byte(id),
	}
}

// String returns the ID as a string using DefaultFormat.
func (id ID) String() string {
	return id.Format(DefaultFormat)
}

// Format returns the ID as a string in the specified format.
// If no format is provided, uses DefaultFormat.
func (id ID) Format(f ...Format) string {
	format := DefaultFormat
	if len(f) > 0 {
		format = f[0]
	}
	id = obfuscate(id)
	switch format {
	case FormatBase58:
		return base58.Encode(int64(id))
	case FormatDecimal:
		return strconv.FormatInt(int64(id), 10)
	case FormatBase64:
		return base64.StdEncoding.EncodeToString(id.Bytes())
	case FormatHash:
		return strconv.FormatUint(uint64(id), 16)
	default:
		return crockford.Encode(int64(id))
	}
}

// Timestamp extracts the creation time from the ID.
func (id ID) Timestamp() time.Time {
	timeShift := SeqBits + NodeBits
	µs := (int64(id) >> timeShift) + Epoch
	return time.UnixMicro(µs)
}

// Node extracts the node ID component from the ID.
func (id ID) Node() int64 {
	nodeMax := int64((1 << NodeBits) - 1)
	return (int64(id) >> SeqBits) & nodeMax
}

// Seq extracts the sequence number component from the ID.
func (id ID) Seq() int64 {
	seqMask := int64((1 << SeqBits) - 1)
	return int64(id) & seqMask
}

// MarshalText implements encoding.TextMarshaler
func (id ID) MarshalText() ([]byte, error) {
	return []byte(id.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler
func (id *ID) UnmarshalText(b []byte) error {
	parsed, err := Parse(string(b))
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// MarshalJSON implements json.Marshaler
func (id ID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + id.String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (id *ID) UnmarshalJSON(b []byte) error {
	// Handle null
	if string(b) == "null" {
		*id = Nil
		return nil
	}
	// Handle numeric value
	if len(b) > 0 && b[0] != '"' {
		n, err := strconv.ParseInt(string(b), 10, 64)
		if err != nil {
			return errors.New("usid: invalid JSON value")
		}
		*id = deobfuscate(ID(n))
		return nil
	}
	// Handle quoted string
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return errors.New("usid: invalid JSON string")
	}
	return id.UnmarshalText(b[1 : len(b)-1])
}

// Value implements driver.Valuer for database storage
func (id ID) Value() (driver.Value, error) {
	return int64(id), nil
}

// Scan implements sql.Scanner for database retrieval
func (id *ID) Scan(src interface{}) error {
	if src == nil {
		*id = Nil
		return nil
	}
	switch v := src.(type) {
	case ID:
		*id = v
		return nil
	case int64:
		*id = ID(v)
		return nil
	case []byte:
		return id.UnmarshalText(v)
	case string:
		return id.UnmarshalText([]byte(v))
	default:
		return fmt.Errorf("usid: cannot scan %T", src)
	}
}

// Parse parses a string into an ID using DefaultFormat.
func Parse(s string) (ID, error) {
	switch DefaultFormat {
	case FormatBase58:
		return ParseBase58(s)
	case FormatDecimal:
		return ParseDecimal(s)
	case FormatBase64:
		return ParseBase64(s)
	case FormatHash:
		return ParseHash(s)
	default:
		return ParseCrockford(s)
	}
}

// ParseCrockford parses a Crockford Base32-encoded string into an ID.
func ParseCrockford(s string) (ID, error) {
	if len(s) == 0 {
		return Nil, errors.New("usid: empty string")
	}
	n, err := crockford.Decode(s)
	if err != nil {
		return Nil, err
	}
	return deobfuscate(ID(n)), nil
}

// ParseBase58 parses a base58-encoded string into an ID.
func ParseBase58(s string) (ID, error) {
	if len(s) == 0 {
		return Nil, errors.New("usid: empty string")
	}
	n, err := base58.Decode(s)
	if err != nil {
		return Nil, err
	}
	return deobfuscate(ID(n)), nil
}

// ParseBase64 parses a base64-encoded string into an ID.
func ParseBase64(s string) (ID, error) {
	if len(s) == 0 {
		return Nil, errors.New("usid: empty string")
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return Nil, fmt.Errorf("usid: invalid base64: %w", err)
	}
	id, err := FromBytes(b)
	if err != nil {
		return Nil, err
	}
	return deobfuscate(id), nil
}

// ParseHash parses a hex-encoded string into an ID.
func ParseHash(s string) (ID, error) {
	if len(s) == 0 {
		return Nil, errors.New("usid: empty string")
	}
	if !isHex(s) {
		return Nil, errors.New("usid: invalid hex string")
	}
	b, err := hexDecode(s)
	if err != nil {
		return Nil, err
	}
	id, err := FromBytes(b)
	if err != nil {
		return Nil, err
	}
	return deobfuscate(id), nil
}

// ParseDecimal parses a decimal string into an ID.
func ParseDecimal(s string) (ID, error) {
	if len(s) == 0 {
		return Nil, errors.New("usid: empty string")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return Nil, fmt.Errorf("usid: invalid decimal: %w", err)
	}
	return deobfuscate(ID(n)), nil
}

// Parse parses a string into the ID receiver.
func (id *ID) Parse(s string) error {
	parsed, err := Parse(s)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func hexDecode(s string) ([]byte, error) {
	if len(s) == 0 || len(s) > 16 {
		return nil, errors.New("usid: hex string must be 1-16 characters")
	}
	n, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return nil, errors.New("usid: invalid hex character")
	}
	b := make([]byte, 8)
	b[0] = byte(n >> 56)
	b[1] = byte(n >> 48)
	b[2] = byte(n >> 40)
	b[3] = byte(n >> 32)
	b[4] = byte(n >> 24)
	b[5] = byte(n >> 16)
	b[6] = byte(n >> 8)
	b[7] = byte(n)
	return b, nil
}

// FromString returns an ID parsed from the input string.
// Alias for Parse.
func FromString(s string) (ID, error) {
	return Parse(s)
}

// FromStringOrNil returns an ID parsed from the input string.
// Returns Nil on error.
func FromStringOrNil(s string) ID {
	id, err := Parse(s)
	if err != nil {
		return Nil
	}
	return id
}

// FromBytes returns an ID from an 8-byte big-endian slice.
func FromBytes(b []byte) (ID, error) {
	if len(b) != 8 {
		return Nil, fmt.Errorf("usid: ID must be exactly 8 bytes, got %d", len(b))
	}
	return ID(int64(b[0])<<56 | int64(b[1])<<48 | int64(b[2])<<40 | int64(b[3])<<32 |
		int64(b[4])<<24 | int64(b[5])<<16 | int64(b[6])<<8 | int64(b[7])), nil
}

// FromBytesOrNil returns an ID from an 8-byte slice.
// Returns Nil on error.
func FromBytesOrNil(b []byte) ID {
	id, err := FromBytes(b)
	if err != nil {
		return Nil
	}
	return id
}

// FromInt64 returns an ID from an int64.
func FromInt64(n int64) ID {
	return ID(n)
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (id ID) MarshalBinary() ([]byte, error) {
	return id.Bytes(), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (id *ID) UnmarshalBinary(data []byte) error {
	parsed, err := FromBytes(data)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// GobEncode implements gob.GobEncoder.
func (id ID) GobEncode() ([]byte, error) {
	return id.MarshalBinary()
}

// GobDecode implements gob.GobDecoder.
func (id *ID) GobDecode(data []byte) error {
	return id.UnmarshalBinary(data)
}

// Must panics if err is not nil
func Must(id ID, err error) ID {
	if err != nil {
		panic(err)
	}
	return id
}

// Generator produces unique IDs for a specific node.
// Create with NewGenerator and call Generate to produce IDs.
type Generator struct {
	node      int64
	state     atomic.Uint64
	seqMask   int64
	nodeShift uint8
	timeShift uint8
}
