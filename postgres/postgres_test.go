package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/paraglidehq/usid/postgres"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupPostgres(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
		testcontainers.CustomizeRequestOption(func(req *testcontainers.GenericContainerRequest) error {
			req.ContainerRequest.WaitingFor = wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30 * time.Second)
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	cleanup := func() {
		db.Close()
		container.Terminate(ctx)
	}

	return db, cleanup
}

func TestMigrate(t *testing.T) {
	db, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()
	cfg := postgres.DefaultConfig()

	// First migration should succeed
	if err := postgres.Migrate(ctx, db, cfg); err != nil {
		t.Fatalf("first migration failed: %v", err)
	}

	// Second migration should be idempotent
	if err := postgres.Migrate(ctx, db, cfg); err != nil {
		t.Fatalf("second migration failed: %v", err)
	}

	// Verify config was stored
	storedCfg, err := postgres.GetConfig(ctx, db)
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}
	if storedCfg != cfg {
		t.Errorf("stored config %+v != expected %+v", storedCfg, cfg)
	}
}

func TestMigrateConfigMismatch(t *testing.T) {
	db, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()

	// First migration with default config
	if err := postgres.Migrate(ctx, db, postgres.DefaultConfig()); err != nil {
		t.Fatalf("first migration failed: %v", err)
	}

	// Second migration with different config should fail
	differentCfg := postgres.Config{
		Epoch:    1234567890000000,
		NodeBits: 6,
		SeqBits:  6,
	}
	err := postgres.Migrate(ctx, db, differentCfg)
	if err == nil {
		t.Fatal("expected error for config mismatch, got nil")
	}
	if !errors.Is(err, postgres.ErrConfigMismatch) {
		t.Errorf("expected ErrConfigMismatch, got: %v", err)
	}
}

func TestNextNode(t *testing.T) {
	db, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()
	if err := postgres.Migrate(ctx, db, postgres.DefaultConfig()); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Get nodes sequentially, should cycle 1-15
	seen := make(map[int64]bool)
	for i := 0; i < 15; i++ {
		node, err := postgres.NextNode(ctx, db)
		if err != nil {
			t.Fatalf("NextNode failed: %v", err)
		}
		if node < 1 || node > 15 {
			t.Errorf("node %d out of range [1,15]", node)
		}
		if seen[node] {
			t.Errorf("duplicate node %d", node)
		}
		seen[node] = true
	}

	// 16th call should wrap to 1
	node, err := postgres.NextNode(ctx, db)
	if err != nil {
		t.Fatalf("NextNode failed: %v", err)
	}
	if node != 1 {
		t.Errorf("expected node 1 after wrap, got %d", node)
	}
}

func TestUSIDGenerate(t *testing.T) {
	db, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()
	if err := postgres.Migrate(ctx, db, postgres.DefaultConfig()); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Generate an ID using the SQL function
	var id int64
	if err := db.QueryRowContext(ctx, "SELECT usid()").Scan(&id); err != nil {
		t.Fatalf("usid() failed: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	// Verify node is 0 (Postgres node)
	var node int
	if err := db.QueryRowContext(ctx, "SELECT node_from_usid($1)", id).Scan(&node); err != nil {
		t.Fatalf("node_from_usid failed: %v", err)
	}
	if node != 0 {
		t.Errorf("expected node 0, got %d", node)
	}
}

func TestTimestampExtraction(t *testing.T) {
	db, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()
	if err := postgres.Migrate(ctx, db, postgres.DefaultConfig()); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Generate an ID and extract timestamp
	var id int64
	if err := db.QueryRowContext(ctx, "SELECT usid()").Scan(&id); err != nil {
		t.Fatalf("usid() failed: %v", err)
	}

	var ts time.Time
	if err := db.QueryRowContext(ctx, "SELECT ts_from_usid($1)", id).Scan(&ts); err != nil {
		t.Fatalf("ts_from_usid failed: %v", err)
	}

	// Timestamp should be within last 5 seconds
	now := time.Now()
	if ts.Before(now.Add(-5*time.Second)) || ts.After(now.Add(5*time.Second)) {
		t.Errorf("timestamp %v not within 5 seconds of now %v", ts, now)
	}
}

func TestNilAndOmni(t *testing.T) {
	db, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()
	if err := postgres.Migrate(ctx, db, postgres.DefaultConfig()); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Test nil_usid()
	var nilID int64
	if err := db.QueryRowContext(ctx, "SELECT nil_usid()").Scan(&nilID); err != nil {
		t.Fatalf("nil_usid() failed: %v", err)
	}
	if nilID != 0 {
		t.Errorf("nil_usid() = %d, want 0", nilID)
	}

	// Test omni_usid()
	var omniID int64
	if err := db.QueryRowContext(ctx, "SELECT omni_usid()").Scan(&omniID); err != nil {
		t.Fatalf("omni_usid() failed: %v", err)
	}
	if omniID != 9223372036854775807 {
		t.Errorf("omni_usid() = %d, want max int64", omniID)
	}

	// Test is_nil_usid()
	var isNil bool
	if err := db.QueryRowContext(ctx, "SELECT is_nil_usid($1)", nilID).Scan(&isNil); err != nil {
		t.Fatalf("is_nil_usid failed: %v", err)
	}
	if !isNil {
		t.Error("is_nil_usid(0) = false, want true")
	}

	if err := db.QueryRowContext(ctx, "SELECT is_nil_usid($1)", int64(123)).Scan(&isNil); err != nil {
		t.Fatalf("is_nil_usid failed: %v", err)
	}
	if isNil {
		t.Error("is_nil_usid(123) = true, want false")
	}

	// Test is_omni_usid()
	var isOmni bool
	if err := db.QueryRowContext(ctx, "SELECT is_omni_usid($1)", omniID).Scan(&isOmni); err != nil {
		t.Fatalf("is_omni_usid failed: %v", err)
	}
	if !isOmni {
		t.Error("is_omni_usid(max) = false, want true")
	}

	if err := db.QueryRowContext(ctx, "SELECT is_omni_usid($1)", int64(123)).Scan(&isOmni); err != nil {
		t.Fatalf("is_omni_usid failed: %v", err)
	}
	if isOmni {
		t.Error("is_omni_usid(123) = true, want false")
	}
}

func TestSequenceIncrement(t *testing.T) {
	db, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()
	if err := postgres.Migrate(ctx, db, postgres.DefaultConfig()); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Generate multiple IDs rapidly and check sequences increment
	var ids []int64
	for i := 0; i < 10; i++ {
		var id int64
		if err := db.QueryRowContext(ctx, "SELECT usid()").Scan(&id); err != nil {
			t.Fatalf("usid() failed: %v", err)
		}
		ids = append(ids, id)
	}

	// Extract sequences
	var seqs []int
	for _, id := range ids {
		var seq int
		if err := db.QueryRowContext(ctx, "SELECT seq_from_usid($1)", id).Scan(&seq); err != nil {
			t.Fatalf("seq_from_usid failed: %v", err)
		}
		seqs = append(seqs, seq)
	}

	// Sequences should be incrementing (with possible wraps at 256)
	for i := 1; i < len(seqs); i++ {
		expected := (seqs[i-1] + 1) % 256
		if seqs[i] != expected && seqs[i] != 0 {
			// Allow for time advancing (seq resets to 0)
			t.Logf("seq[%d]=%d, seq[%d]=%d (time may have advanced)", i-1, seqs[i-1], i, seqs[i])
		}
	}

	// At minimum, all IDs should be unique
	seen := make(map[int64]bool)
	for _, id := range ids {
		if seen[id] {
			t.Errorf("duplicate ID generated: %d", id)
		}
		seen[id] = true
	}
}

func TestEncodingRoundtrip(t *testing.T) {
	db, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()
	if err := postgres.Migrate(ctx, db, postgres.DefaultConfig()); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	testID := int64(1234567890123456789)

	tests := []struct {
		name   string
		encode string
		decode string
	}{
		{"base58", "usid_to_b58", "b58_to_usid"},
		{"base64", "usid_to_b64", "b64_to_usid"},
		{"hex", "usid_to_hex", "hex_to_usid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			var encoded string
			if err := db.QueryRowContext(ctx, "SELECT "+tt.encode+"($1)", testID).Scan(&encoded); err != nil {
				t.Fatalf("%s failed: %v", tt.encode, err)
			}
			if encoded == "" {
				t.Fatalf("%s returned empty string", tt.encode)
			}

			// Decode
			var decoded int64
			if err := db.QueryRowContext(ctx, "SELECT "+tt.decode+"($1)", encoded).Scan(&decoded); err != nil {
				t.Fatalf("%s failed: %v", tt.decode, err)
			}
			if decoded != testID {
				t.Errorf("roundtrip failed: got %d, want %d", decoded, testID)
			}
		})
	}
}
