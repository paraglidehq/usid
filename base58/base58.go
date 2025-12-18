// Package base58 provides Base58 encoding and decoding for int64 values.
// It uses the Bitcoin alphabet which excludes 0, O, I, and l to avoid ambiguity.
package base58

import "errors"

var encode = [58]byte{
	'1', '2', '3', '4', '5', '6', '7', '8', '9', 'A',
	'B', 'C', 'D', 'E', 'F', 'G', 'H', 'J', 'K', 'L',
	'M', 'N', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W',
	'X', 'Y', 'Z', 'a', 'b', 'c', 'd', 'e', 'f', 'g',
	'h', 'i', 'j', 'k', 'm', 'n', 'o', 'p', 'q', 'r',
	's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
}

var decode = [128]int64{
	'1': 0, '2': 1, '3': 2, '4': 3, '5': 4,
	'6': 5, '7': 6, '8': 7, '9': 8, 'A': 9,
	'B': 10, 'C': 11, 'D': 12, 'E': 13, 'F': 14,
	'G': 15, 'H': 16, 'J': 17, 'K': 18, 'L': 19,
	'M': 20, 'N': 21, 'P': 22, 'Q': 23, 'R': 24,
	'S': 25, 'T': 26, 'U': 27, 'V': 28, 'W': 29,
	'X': 30, 'Y': 31, 'Z': 32, 'a': 33, 'b': 34,
	'c': 35, 'd': 36, 'e': 37, 'f': 38, 'g': 39,
	'h': 40, 'i': 41, 'j': 42, 'k': 43, 'm': 44,
	'n': 45, 'o': 46, 'p': 47, 'q': 48, 'r': 49,
	's': 50, 't': 51, 'u': 52, 'v': 53, 'w': 54,
	'x': 55, 'y': 56, 'z': 57,
}

// ErrInvalidBase58 is returned when decoding a string with invalid Base58 characters.
var ErrInvalidBase58 = errors.New("usid: invalid base58 character")

// Encode returns the Base58 encoding of the given int64.
func Encode(id int64) string {
	if id == 0 {
		return "1"
	}
	var buf [11]byte
	i := 10
	for id > 0 {
		buf[i] = encode[id%58]
		id /= 58
		i--
	}
	return string(buf[i+1:])
}

// Decode parses a Base58-encoded string and returns the int64 value.
// Returns ErrInvalidBase58 if the string contains invalid characters.
func Decode(s string) (int64, error) {
	var id int64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 128 {
			return 0, ErrInvalidBase58
		}
		v := decode[c]
		if v == 0 && c != '1' {
			return 0, ErrInvalidBase58
		}
		id = id*58 + v
	}
	return id, nil
}
