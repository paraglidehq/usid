package usid

import (
	"bytes"
	"encoding/gob"
	"testing"
)

// codecTestID is a sample ID for codec testing
var codecTestID = ID(1234567890123456789)
var codecTestBytes = codecTestID.Bytes()

func TestFromBytes(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		got, err := FromBytes(codecTestBytes)
		if err != nil {
			t.Fatal(err)
		}
		if got != codecTestID {
			t.Fatalf("FromBytes(%x) = %v, want %v", codecTestBytes, got, codecTestID)
		}
	})
	t.Run("Invalid", func(t *testing.T) {
		invalid := [][]byte{
			{},
			{1, 2, 3},
			{1, 2, 3, 4, 5, 6, 7},
			{1, 2, 3, 4, 5, 6, 7, 8, 9},
		}
		for _, b := range invalid {
			got, err := FromBytes(b)
			if err == nil {
				t.Fatalf("FromBytes(%x): want err != nil, got %v", b, got)
			}
		}
	})
}

func TestFromBytesOrNil(t *testing.T) {
	t.Run("Invalid", func(t *testing.T) {
		b := []byte{4, 8, 15}
		got := FromBytesOrNil(b)
		if got != Nil {
			t.Errorf("FromBytesOrNil(%x): got %v, want %v", b, got, Nil)
		}
	})
	t.Run("Valid", func(t *testing.T) {
		got := FromBytesOrNil(codecTestBytes)
		if got != codecTestID {
			t.Errorf("FromBytesOrNil(%x): got %v, want %v", codecTestBytes, got, codecTestID)
		}
	})
}

func TestParse(t *testing.T) {
	// Parse uses DefaultFormat (base58 by default)
	s := codecTestID.Format(FormatBase58)
	got, err := Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	if got != codecTestID {
		t.Errorf("Parse(%q): got %v, want %v", s, got, codecTestID)
	}
}

func TestParseBase58(t *testing.T) {
	s := codecTestID.Format(FormatBase58)
	got, err := ParseBase58(s)
	if err != nil {
		t.Fatal(err)
	}
	if got != codecTestID {
		t.Errorf("ParseBase58(%q): got %v, want %v", s, got, codecTestID)
	}
}

func TestParseBase64(t *testing.T) {
	s := codecTestID.Format(FormatBase64)
	got, err := ParseBase64(s)
	if err != nil {
		t.Fatal(err)
	}
	if got != codecTestID {
		t.Errorf("ParseBase64(%q): got %v, want %v", s, got, codecTestID)
	}
}

func TestParseHash(t *testing.T) {
	s := codecTestID.Format(FormatHash)
	got, err := ParseHash(s)
	if err != nil {
		t.Fatal(err)
	}
	if got != codecTestID {
		t.Errorf("ParseHash(%q): got %v, want %v", s, got, codecTestID)
	}
}

func TestParseDecimal(t *testing.T) {
	s := codecTestID.Format(FormatDecimal)
	got, err := ParseDecimal(s)
	if err != nil {
		t.Fatal(err)
	}
	if got != codecTestID {
		t.Errorf("ParseDecimal(%q): got %v, want %v", s, got, codecTestID)
	}
}

func TestParseEmpty(t *testing.T) {
	fns := []struct {
		name string
		fn   func(string) (ID, error)
	}{
		{"ParseBase58", ParseBase58},
		{"ParseBase64", ParseBase64},
		{"ParseHash", ParseHash},
		{"ParseDecimal", ParseDecimal},
	}
	for _, tt := range fns {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.fn("")
			if err == nil {
				t.Errorf("%s(empty): want err != nil", tt.name)
			}
		})
	}
}

func TestFromStringOrNil(t *testing.T) {
	t.Run("Invalid", func(t *testing.T) {
		got := FromStringOrNil("invalid!!!")
		if got != Nil {
			t.Errorf("FromStringOrNil(invalid): got %v, want Nil", got)
		}
	})
	t.Run("Valid", func(t *testing.T) {
		s := codecTestID.Format(FormatBase58)
		got := FromStringOrNil(s)
		if got != codecTestID {
			t.Errorf("FromStringOrNil(%q): got %v, want %v", s, got, codecTestID)
		}
	})
}

func TestFromInt64(t *testing.T) {
	got := FromInt64(1234567890123456789)
	if got != codecTestID {
		t.Errorf("FromInt64: got %v, want %v", got, codecTestID)
	}
}

func TestIDParseMethod(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		var id ID
		err := id.Parse(codecTestID.Format(FormatBase58))
		if err != nil {
			t.Fatal(err)
		}
		if id != codecTestID {
			t.Errorf("ID.Parse: got %v, want %v", id, codecTestID)
		}
	})
	t.Run("Invalid", func(t *testing.T) {
		var id ID
		err := id.Parse("invalid!!!")
		if err == nil {
			t.Error("ID.Parse(invalid): want err != nil")
		}
	})
}

func TestMarshalBinary(t *testing.T) {
	got, err := codecTestID.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, codecTestBytes) {
		t.Fatalf("%v.MarshalBinary() = %x, want %x", codecTestID, got, codecTestBytes)
	}
}

func TestUnmarshalBinary(t *testing.T) {
	var got ID
	err := got.UnmarshalBinary(codecTestBytes)
	if err != nil {
		t.Fatal(err)
	}
	if got != codecTestID {
		t.Errorf("UnmarshalBinary: got %v, want %v", got, codecTestID)
	}
}

func TestGobEncode(t *testing.T) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(codecTestID); err != nil {
		t.Fatal(err)
	}

	var got ID
	dec := gob.NewDecoder(&buf)
	if err := dec.Decode(&got); err != nil {
		t.Fatal(err)
	}

	if got != codecTestID {
		t.Errorf("Gob roundtrip: got %v, want %v", got, codecTestID)
	}
}

func TestMarshalText(t *testing.T) {
	got, err := codecTestID.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(codecTestID.String())
	if !bytes.Equal(got, want) {
		t.Errorf("%v.MarshalText(): got %s, want %s", codecTestID, got, want)
	}
}

func TestUnmarshalText(t *testing.T) {
	// UnmarshalText uses Parse which uses DefaultFormat (base58)
	var got ID
	err := got.UnmarshalText([]byte(codecTestID.Format(FormatBase58)))
	if err != nil {
		t.Fatal(err)
	}
	if got != codecTestID {
		t.Errorf("UnmarshalText: got %v, want %v", got, codecTestID)
	}
}

func TestMarshalJSON(t *testing.T) {
	got, err := codecTestID.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	want := `"` + codecTestID.String() + `"`
	if string(got) != want {
		t.Errorf("MarshalJSON: got %s, want %s", got, want)
	}
}

func TestUnmarshalJSON(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		var got ID
		err := got.UnmarshalJSON([]byte(`"` + codecTestID.String() + `"`))
		if err != nil {
			t.Fatal(err)
		}
		if got != codecTestID {
			t.Errorf("UnmarshalJSON(string): got %v, want %v", got, codecTestID)
		}
	})
	t.Run("Numeric", func(t *testing.T) {
		var got ID
		err := got.UnmarshalJSON([]byte("1234567890123456789"))
		if err != nil {
			t.Fatal(err)
		}
		if got != codecTestID {
			t.Errorf("UnmarshalJSON(numeric): got %v, want %v", got, codecTestID)
		}
	})
	t.Run("Null", func(t *testing.T) {
		var got ID
		err := got.UnmarshalJSON([]byte("null"))
		if err != nil {
			t.Fatal(err)
		}
		if got != Nil {
			t.Errorf("UnmarshalJSON(null): got %v, want Nil", got)
		}
	})
	t.Run("Invalid", func(t *testing.T) {
		var got ID
		err := got.UnmarshalJSON([]byte("not-json"))
		if err == nil {
			t.Errorf("UnmarshalJSON(invalid): want err, got %v", got)
		}
	})
}

func TestIDFormat(t *testing.T) {
	tests := []struct {
		format Format
		name   string
		parse  func(string) (ID, error)
	}{
		{FormatBase58, "Base58", ParseBase58},
		{FormatDecimal, "Decimal", ParseDecimal},
		{FormatHash, "Hash", ParseHash},
		{FormatBase64, "Base64", ParseBase64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := codecTestID.Format(tt.format)
			if s == "" {
				t.Errorf("Format(%s) returned empty string", tt.format)
			}
			// Verify roundtrip with matching parse function
			got, err := tt.parse(s)
			if err != nil {
				t.Errorf("%s parse(%q) failed: %v", tt.name, s, err)
			}
			if got != codecTestID {
				t.Errorf("Roundtrip failed: got %v, want %v", got, codecTestID)
			}
		})
	}
}

func TestMust(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		got := Must(FromString(codecTestID.Format(FormatBase58)))
		if got != codecTestID {
			t.Errorf("Must: got %v, want %v", got, codecTestID)
		}
	})
	t.Run("Panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Must did not panic on error")
			}
		}()
		Must(FromString("invalid!!!"))
	})
}

func BenchmarkFromString(b *testing.B) {
	b.Run("Decimal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			FromString("1234567890123456789")
		}
	})
	b.Run("Base58", func(b *testing.B) {
		s := codecTestID.Format(FormatBase58)
		for i := 0; i < b.N; i++ {
			FromString(s)
		}
	})
}

func BenchmarkFormat(b *testing.B) {
	b.Run("Base58", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			codecTestID.Format(FormatBase58)
		}
	})
	b.Run("Decimal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			codecTestID.Format(FormatDecimal)
		}
	})
	b.Run("Hash", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			codecTestID.Format(FormatHash)
		}
	})
}

func BenchmarkMarshalBinary(b *testing.B) {
	for i := 0; i < b.N; i++ {
		codecTestID.MarshalBinary()
	}
}

func BenchmarkMarshalText(b *testing.B) {
	for i := 0; i < b.N; i++ {
		codecTestID.MarshalText()
	}
}
