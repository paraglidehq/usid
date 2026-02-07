package usid

import (
	"sync"
	"testing"
	"time"
)

func TestID(t *testing.T) {
	t.Run("IsNil", testIDIsNil)
	t.Run("Bytes", testIDBytes)
	t.Run("String", testIDString)
	t.Run("Format", testIDFormats)
	t.Run("Timestamp", testIDTimestamp)
	t.Run("Node", testIDNode)
	t.Run("Seq", testIDSeq)
}

func testIDIsNil(t *testing.T) {
	var id ID
	if !id.IsNil() {
		t.Errorf("zero ID.IsNil() = false, want true")
	}
	if !Nil.IsNil() {
		t.Errorf("Nil.IsNil() = false, want true")
	}
	id = New()
	if id.IsNil() {
		t.Errorf("New().IsNil() = true, want false")
	}
}

func testIDBytes(t *testing.T) {
	id := ID(0x1122334455667788)
	got := id.Bytes()
	want := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("Bytes()[%d] = %x, want %x", i, got[i], want[i])
		}
	}
}

func testIDString(t *testing.T) {
	id := New()
	s := id.String()
	if s == "" {
		t.Error("String() returned empty string")
	}
	// Should be base58 by default
	parsed, err := FromString(s)
	if err != nil {
		t.Errorf("FromString(%q) failed: %v", s, err)
	}
	if parsed != id {
		t.Errorf("Roundtrip failed: got %v, want %v", parsed, id)
	}
}

func testIDFormats(t *testing.T) {
	id := New()

	formats := []Format{FormatCrockford, FormatBase58, FormatDecimal, FormatHash, FormatBase64}
	for _, f := range formats {
		s := id.Format(f)
		if s == "" {
			t.Errorf("Format(%s) returned empty string", f)
		}
	}
}

func testIDTimestamp(t *testing.T) {
	before := time.Now()
	id := New()
	after := time.Now()

	ts := id.Timestamp()
	if ts.Before(before.Add(-time.Second)) || ts.After(after.Add(time.Second)) {
		t.Errorf("Timestamp() = %v, expected between %v and %v", ts, before, after)
	}
}

func testIDNode(t *testing.T) {
	SetNodeID(5)
	id := New()
	node := id.Node()
	if node != 5 {
		t.Errorf("Node() = %d, want 5", node)
	}
	SetNodeID(1) // Reset
}

func testIDSeq(t *testing.T) {
	id := New()
	seq := id.Seq()
	if seq < 0 || seq > 255 {
		t.Errorf("Seq() = %d, out of range [0, 255]", seq)
	}
}

func TestNew(t *testing.T) {
	id := New()
	if id.IsNil() {
		t.Error("New() returned Nil ID")
	}

	// Should have valid timestamp
	ts := id.Timestamp()
	now := time.Now()
	if ts.Before(now.Add(-time.Hour)) || ts.After(now.Add(time.Second)) {
		t.Errorf("New().Timestamp() = %v, unreasonable", ts)
	}
}

func TestGenerator(t *testing.T) {
	gen := NewGenerator(3)
	id := gen.Generate()

	if id.IsNil() {
		t.Error("Generator.Generate() returned Nil ID")
	}

	node := id.Node()
	if node != 3 {
		t.Errorf("Generated ID has node %d, want 3", node)
	}
}

func TestConcurrentGeneration(t *testing.T) {
	const numGoroutines = 100
	const numIDs = 100

	// Use a dedicated generator to avoid interference from other tests
	gen := NewGenerator(0)

	var wg sync.WaitGroup
	results := make([][]ID, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ids := make([]ID, numIDs)
			for j := 0; j < numIDs; j++ {
				ids[j] = gen.Generate()
			}
			results[idx] = ids
		}(i)
	}

	wg.Wait()

	// Check all IDs are unique across all goroutines
	seen := make(map[ID]bool)
	for i, ids := range results {
		for j, id := range ids {
			if seen[id] {
				t.Errorf("Duplicate ID found: %s (goroutine %d, index %d)", id, i, j)
			}
			seen[id] = true
		}
	}
}

func TestRapidGeneration(t *testing.T) {
	// Generate many IDs rapidly to test sequence behavior
	ids := make([]ID, 1000)
	for i := 0; i < 1000; i++ {
		ids[i] = New()
	}

	// All should be unique
	seen := make(map[ID]bool)
	for i, id := range ids {
		if seen[id] {
			t.Errorf("Duplicate ID at index %d: %s", i, id)
		}
		seen[id] = true
	}

	// Timestamps should be monotonic (non-decreasing)
	var lastTS time.Time
	for i, id := range ids {
		ts := id.Timestamp()
		if ts.Before(lastTS) {
			t.Errorf("Timestamp went backwards at index %d: %v < %v", i, ts, lastTS)
		}
		lastTS = ts
	}
}

func TestIDUniqueness(t *testing.T) {
	seen := make(map[ID]bool)

	for i := 0; i < 10000; i++ {
		id := New()
		if seen[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestIDComponents(t *testing.T) {
	SetNodeID(7)
	defer SetNodeID(1)

	id := New()

	// Check components are within valid ranges
	node := id.Node()
	if node != 7 {
		t.Errorf("Node() = %d, want 7", node)
	}

	seq := id.Seq()
	if seq < 0 || seq > 255 {
		t.Errorf("Seq() = %d, out of range", seq)
	}

	ts := id.Timestamp()
	now := time.Now()
	if ts.Before(now.Add(-time.Second)) || ts.After(now.Add(time.Second)) {
		t.Errorf("Timestamp() = %v, unreasonable (now=%v)", ts, now)
	}

	// Verify Int64 roundtrip
	i64 := id.Int64()
	if ID(i64) != id {
		t.Errorf("Int64() roundtrip failed")
	}
}

func BenchmarkNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = New()
	}
}

func BenchmarkNewParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = New()
		}
	})
}

func BenchmarkIDString(b *testing.B) {
	id := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id.String()
	}
}

func BenchmarkIDTimestamp(b *testing.B) {
	id := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id.Timestamp()
	}
}
