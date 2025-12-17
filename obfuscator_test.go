package usid

import (
	"encoding/json"
	"testing"
)

func TestObfuscation(t *testing.T) {
	// Set up obfuscator for this test
	key := int64(0x123456789ABCDEF0)
	DefaultObfuscator = NewObfuscator(key)
	defer func() { DefaultObfuscator = nil }()

	id := New()

	// String should be obfuscated
	s := id.String()
	parsed, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if parsed != id {
		t.Errorf("roundtrip failed: got %d, want %d", parsed, id)
	}

	// Without obfuscation, parsing same string should give different result
	DefaultObfuscator = nil
	parsedRaw, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if parsedRaw == id {
		t.Error("raw parse should differ from obfuscated ID")
	}

	// Re-enable for rest of tests
	DefaultObfuscator = NewObfuscator(key)

	// Test all formats roundtrip
	formats := []Format{FormatBase58, FormatDecimal, FormatHash, FormatBase64}
	for _, f := range formats {
		DefaultFormat = f
		s := id.Format(f)
		parsed, err := Parse(s)
		if err != nil {
			t.Fatalf("Parse(%s) failed for format %s: %v", s, f, err)
		}
		if parsed != id {
			t.Errorf("roundtrip failed for format %s: got %d, want %d", f, parsed, id)
		}
	}
	DefaultFormat = FormatBase58 // restore
}

func TestObfuscationJSON(t *testing.T) {
	key := int64(0x1EADBEEFCAFEBABE)
	DefaultObfuscator = NewObfuscator(key)
	defer func() { DefaultObfuscator = nil }()

	id := New()

	// Marshal
	data, err := json.Marshal(id)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal
	var parsed ID
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if parsed != id {
		t.Errorf("JSON roundtrip failed: got %d, want %d", parsed, id)
	}
}

func TestObfuscationInternalValuesUnchanged(t *testing.T) {
	key := int64(0x7EDCBA9876543210)
	DefaultObfuscator = NewObfuscator(key)
	defer func() { DefaultObfuscator = nil }()

	SetNodeID(5)
	defer SetNodeID(1)

	id := New()

	// Internal values should be unobfuscated
	if id.Node() != 5 {
		t.Errorf("Node() = %d, want 5", id.Node())
	}

	// Int64 should be raw
	raw := id.Int64()
	if ID(raw) != id {
		t.Errorf("Int64() roundtrip failed")
	}

	// Bytes should be raw
	b := id.Bytes()
	restored, _ := FromBytes(b)
	// FromBytes doesn't go through parse, so no deobfuscation
	if restored != id {
		t.Errorf("Bytes() roundtrip failed")
	}
}

func TestObfuscatorMethods(t *testing.T) {
	key := int64(0x1111111111111111)
	o := NewObfuscator(key)

	id := ID(0x2222222222222222)
	obf := o.Obfuscate(id)
	deobf := o.Deobfuscate(obf)

	if deobf != id {
		t.Errorf("Deobfuscate(Obfuscate(id)) != id")
	}

	// XOR with known values
	expected := ID(0x2222222222222222 ^ 0x1111111111111111)
	if obf != expected {
		t.Errorf("Obfuscate: got %x, want %x", obf, expected)
	}
}

func TestNoObfuscation(t *testing.T) {
	// Ensure DefaultObfuscator is nil
	DefaultObfuscator = nil

	id := New()
	s := id.String()
	parsed, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if parsed != id {
		t.Errorf("roundtrip failed without obfuscation")
	}
}
