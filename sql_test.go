package usid

import (
	"encoding/json"
	"testing"
)

// testID is a sample ID for testing
var testID = ID(1234567890123456789)

func TestIDSQL(t *testing.T) {
	t.Run("Value", testIDSQLValue)
	t.Run("Scan", func(t *testing.T) {
		t.Run("Int64", testIDSQLScanInt64)
		t.Run("String", testIDSQLScanString)
		t.Run("Bytes", testIDSQLScanBytes)
		t.Run("ID", testIDSQLScanID)
		t.Run("Unsupported", testIDSQLScanUnsupported)
		t.Run("Nil", testIDSQLScanNil)
	})
}

func testIDSQLValue(t *testing.T) {
	v, err := testID.Value()
	if err != nil {
		t.Fatal(err)
	}
	got, ok := v.(int64)
	if !ok {
		t.Fatalf("Value() returned %T, want int64", v)
	}
	if want := testID.Int64(); got != want {
		t.Errorf("Value() == %d, want %d", got, want)
	}
}

func testIDSQLScanInt64(t *testing.T) {
	var got ID
	err := got.Scan(testID.Int64())
	if err != nil {
		t.Fatal(err)
	}
	if got != testID {
		t.Errorf("Scan(%d): got %v, want %v", testID.Int64(), got, testID)
	}
}

func testIDSQLScanString(t *testing.T) {
	s := testID.String()
	var got ID
	err := got.Scan(s)
	if err != nil {
		t.Fatal(err)
	}
	if got != testID {
		t.Errorf("Scan(%q): got %v, want %v", s, got, testID)
	}
}

func testIDSQLScanBytes(t *testing.T) {
	s := testID.String()
	var got ID
	err := got.Scan([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	if got != testID {
		t.Errorf("Scan(%q): got %v, want %v", s, got, testID)
	}
}

func testIDSQLScanID(t *testing.T) {
	var got ID
	err := got.Scan(testID)
	if err != nil {
		t.Fatal(err)
	}
	if got != testID {
		t.Errorf("Scan(ID): got %v, want %v", got, testID)
	}
}

func testIDSQLScanUnsupported(t *testing.T) {
	unsupported := []interface{}{
		true,
		42.5,
	}
	for _, v := range unsupported {
		var got ID
		err := got.Scan(v)
		if err == nil {
			t.Errorf("Scan(%T) succeeded, got %v", v, got)
		}
	}
}

func testIDSQLScanNil(t *testing.T) {
	var got ID
	err := got.Scan(nil)
	if err != nil || !got.IsNil() {
		t.Errorf("Scan(nil) failed, got %v", got)
	}
}

func TestNullID(t *testing.T) {
	t.Run("Value", func(t *testing.T) {
		t.Run("Nil", testNullIDValueNil)
		t.Run("Valid", testNullIDValueValid)
	})

	t.Run("Scan", func(t *testing.T) {
		t.Run("Nil", testNullIDScanNil)
		t.Run("Valid", testNullIDScanValid)
	})

	t.Run("MarshalJSON", func(t *testing.T) {
		t.Run("Nil", testNullIDMarshalJSONNil)
		t.Run("Null", testNullIDMarshalJSONNull)
		t.Run("Valid", testNullIDMarshalJSONValid)
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		t.Run("Null", testNullIDUnmarshalJSONNull)
		t.Run("Valid", testNullIDUnmarshalJSONValid)
		t.Run("Numeric", testNullIDUnmarshalJSONNumeric)
		t.Run("Malformed", testNullIDUnmarshalJSONMalformed)
	})
}

func testNullIDValueNil(t *testing.T) {
	n := NullID{}
	got, err := n.Value()
	if got != nil {
		t.Errorf("null NullID.Value returned non-nil driver.Value")
	}
	if err != nil {
		t.Errorf("null NullID.Value returned non-nil error")
	}
}

func testNullIDValueValid(t *testing.T) {
	n := NullID{
		Valid: true,
		ID:    testID,
	}
	got, err := n.Value()
	if err != nil {
		t.Fatal(err)
	}
	i, ok := got.(int64)
	if !ok {
		t.Errorf("Value() returned %T, want int64", got)
	}
	if i != testID.Int64() {
		t.Errorf("%v.Value() == %d, want %d", n, i, testID.Int64())
	}
}

func testNullIDScanNil(t *testing.T) {
	n := NullID{}
	err := n.Scan(nil)
	if err != nil {
		t.Fatal(err)
	}
	if n.Valid {
		t.Error("NullID is valid after Scan(nil)")
	}
	if n.ID != Nil {
		t.Errorf("NullID.ID is %v after Scan(nil) want Nil", n.ID)
	}
}

func testNullIDScanValid(t *testing.T) {
	n := NullID{}
	err := n.Scan(testID.Int64())
	if err != nil {
		t.Fatal(err)
	}
	if !n.Valid {
		t.Errorf("Valid == false after Scan(%d)", testID.Int64())
	}
	if n.ID != testID {
		t.Errorf("ID == %v after Scan(%d), want %v", n.ID, testID.Int64(), testID)
	}
}

func testNullIDMarshalJSONNil(t *testing.T) {
	n := NullID{Valid: true, ID: Nil}

	data, err := n.MarshalJSON()
	if err != nil {
		t.Fatalf("(%#v).MarshalJSON err want: <nil>, got: %v", n, err)
	}

	dataStr := string(data)
	want := `"0"`
	if dataStr != want {
		t.Fatalf("(%#v).MarshalJSON value want: %s, got: %s", n, want, dataStr)
	}
}

func testNullIDMarshalJSONValid(t *testing.T) {
	n := NullID{
		Valid: true,
		ID:    testID,
	}

	data, err := n.MarshalJSON()
	if err != nil {
		t.Fatalf("(%#v).MarshalJSON err want: <nil>, got: %v", n, err)
	}

	dataStr := string(data)
	want := `"` + testID.String() + `"`
	if dataStr != want {
		t.Fatalf("(%#v).MarshalJSON value want: %s, got: %s", n, want, dataStr)
	}
}

func testNullIDMarshalJSONNull(t *testing.T) {
	n := NullID{}

	data, err := n.MarshalJSON()
	if err != nil {
		t.Fatalf("(%#v).MarshalJSON err want: <nil>, got: %v", n, err)
	}

	dataStr := string(data)
	if dataStr != "null" {
		t.Fatalf("(%#v).MarshalJSON value want: %s, got: %s", n, "null", dataStr)
	}
}

func testNullIDUnmarshalJSONNull(t *testing.T) {
	var n NullID

	data := []byte(`null`)

	if err := json.Unmarshal(data, &n); err != nil {
		t.Fatalf("json.Unmarshal err = %v, want <nil>", err)
	}

	if n.Valid {
		t.Fatalf("n.Valid = true, want false")
	}

	if n.ID != Nil {
		t.Fatalf("n.ID = %v, want %v", n.ID, Nil)
	}
}

func testNullIDUnmarshalJSONValid(t *testing.T) {
	var n NullID

	data := []byte(`"` + testID.String() + `"`)

	if err := json.Unmarshal(data, &n); err != nil {
		t.Fatalf("json.Unmarshal err = %v, want <nil>", err)
	}

	if !n.Valid {
		t.Fatalf("n.Valid = false, want true")
	}

	if n.ID != testID {
		t.Fatalf("n.ID = %v, want %v", n.ID, testID)
	}
}

func testNullIDUnmarshalJSONMalformed(t *testing.T) {
	var n NullID

	// Objects are not valid ID values
	data := []byte(`{"foo": "bar"}`)

	if err := json.Unmarshal(data, &n); err == nil {
		t.Fatal("json.Unmarshal err = <nil>, want error")
	}
}

func testNullIDUnmarshalJSONNumeric(t *testing.T) {
	var n NullID

	data := []byte(`1234567890123456789`)

	if err := json.Unmarshal(data, &n); err != nil {
		t.Fatalf("json.Unmarshal err = %v, want <nil>", err)
	}

	if !n.Valid {
		t.Fatal("n.Valid = false, want true")
	}

	if n.ID != testID {
		t.Fatalf("n.ID = %v, want %v", n.ID, testID)
	}
}

func BenchmarkNullIDMarshalJSON(b *testing.B) {
	b.Run("Valid", func(b *testing.B) {
		n := NullID{ID: testID, Valid: true}
		for i := 0; i < b.N; i++ {
			n.MarshalJSON()
		}
	})
	b.Run("Invalid", func(b *testing.B) {
		n := NullID{Valid: false}
		for i := 0; i < b.N; i++ {
			n.MarshalJSON()
		}
	})
}

func BenchmarkNullIDUnmarshalJSON(b *testing.B) {
	data, _ := json.Marshal(testID)

	b.Run("Valid", func(b *testing.B) {
		var n NullID
		for i := 0; i < b.N; i++ {
			n.UnmarshalJSON(data)
		}
	})
	b.Run("Invalid", func(b *testing.B) {
		invalid := []byte("null")
		var n NullID
		for i := 0; i < b.N; i++ {
			n.UnmarshalJSON(invalid)
		}
	})
}
