// Package crockford provides Crockford Base32 encoding and decoding for int64 values.
// It uses the Crockford alphabet which excludes I, L, O, U to avoid ambiguity.
// Decoding is case-insensitive.
package crockford

import "errors"

var encode = [32]byte{
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
	'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'j', 'k',
	'm', 'n', 'p', 'q', 'r', 's', 't', 'v', 'w', 'x',
	'y', 'z',
}

var decode [128]int64

func init() {
	for i := range decode {
		decode[i] = -1
	}
	for i, c := range encode {
		decode[c] = int64(i)
		// Case-insensitive: map uppercase to same value
		if c >= 'a' && c <= 'z' {
			decode[c-32] = int64(i)
		}
	}
	// Crockford substitutions
	decode['I'] = 1
	decode['i'] = 1
	decode['L'] = 1
	decode['l'] = 1
	decode['O'] = 0
	decode['o'] = 0
}

// ErrInvalid is returned when decoding a string with invalid characters.
var ErrInvalid = errors.New("usid: invalid crockford character")

// Encode returns the Crockford Base32 encoding of the given int64.
func Encode(id int64) string {
	if id == 0 {
		return "0"
	}
	var buf [13]byte // max 13 chars for int64
	i := 12
	for id > 0 {
		buf[i] = encode[id&0x1f]
		id >>= 5
		i--
	}
	return string(buf[i+1:])
}

// Decode parses a Crockford Base32-encoded string and returns the int64 value.
// Decoding is case-insensitive. I and L are treated as 1, O is treated as 0.
// Returns ErrInvalid if the string contains invalid characters.
func Decode(s string) (int64, error) {
	var id int64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '-' {
			continue // Crockford allows hyphens as separators
		}
		if c >= 128 {
			return 0, ErrInvalid
		}
		v := decode[c]
		if v < 0 {
			return 0, ErrInvalid
		}
		id = (id << 5) | v
	}
	return id, nil
}
