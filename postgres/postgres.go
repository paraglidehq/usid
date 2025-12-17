package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Config holds the USID bit layout configuration.
type Config struct {
	Epoch    int64
	NodeBits uint8
	SeqBits  uint8
}

// DefaultConfig returns the default USID configuration.
// Use this unless you've customized usid.Epoch, usid.NodeBits, or usid.SeqBits.
func DefaultConfig() Config {
	return Config{
		Epoch:    1765947799213000, // 2025-12-16 in µs
		NodeBits: 6,
		SeqBits:  6,
	}
}

// Computed values
func (c Config) TimeShift() uint8 { return c.NodeBits + c.SeqBits }
func (c Config) MaxNode() int64   { return (1 << c.NodeBits) - 1 }
func (c Config) MaxSeq() int64    { return (1 << c.SeqBits) - 1 }
func (c Config) NodeMask() int64  { return c.MaxNode() }
func (c Config) SeqMask() int64   { return c.MaxSeq() }

var ErrConfigMismatch = errors.New("usid: database config does not match application config")

// Migrate runs the idempotent USID migration with the given configuration.
// If the database already has a different configuration, returns ErrConfigMismatch.
func Migrate(ctx context.Context, db *sql.DB, cfg Config) error {
	// Create config table
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _usid_config (
			id int PRIMARY KEY DEFAULT 1 CHECK (id = 1),
			epoch bigint NOT NULL,
			node_bits int NOT NULL,
			seq_bits int NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("usid: create config table: %w", err)
	}

	// Check existing config
	var epoch int64
	var nodeBits, seqBits int
	err = db.QueryRowContext(ctx, `SELECT epoch, node_bits, seq_bits FROM _usid_config`).Scan(&epoch, &nodeBits, &seqBits)
	if err == nil {
		// Config exists, validate it matches
		if epoch != cfg.Epoch || uint8(nodeBits) != cfg.NodeBits || uint8(seqBits) != cfg.SeqBits {
			return fmt.Errorf("%w: db has epoch=%d node_bits=%d seq_bits=%d, app has epoch=%d node_bits=%d seq_bits=%d",
				ErrConfigMismatch, epoch, nodeBits, seqBits, cfg.Epoch, cfg.NodeBits, cfg.SeqBits)
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		// Insert config
		_, err = db.ExecContext(ctx, `INSERT INTO _usid_config (epoch, node_bits, seq_bits) VALUES ($1, $2, $3)`,
			cfg.Epoch, cfg.NodeBits, cfg.SeqBits)
		if err != nil {
			return fmt.Errorf("usid: insert config: %w", err)
		}
	} else {
		return fmt.Errorf("usid: read config: %w", err)
	}

	// Generate and run migrations with configured values
	migrations := generateSQL(cfg)
	_, err = db.ExecContext(ctx, migrations)
	if err != nil {
		return fmt.Errorf("usid: run migrations: %w", err)
	}

	return nil
}

// NextNode returns the next available node ID from the database sequence.
// Call once at app startup to get a unique node ID for this instance.
func NextNode(ctx context.Context, db *sql.DB) (int64, error) {
	var node int64
	err := db.QueryRowContext(ctx, "SELECT usid_next_node()").Scan(&node)
	return node, err
}

// GetConfig reads the USID configuration from the database.
func GetConfig(ctx context.Context, db *sql.DB) (Config, error) {
	var cfg Config
	var nodeBits, seqBits int
	err := db.QueryRowContext(ctx, `SELECT epoch, node_bits, seq_bits FROM _usid_config`).Scan(&cfg.Epoch, &nodeBits, &seqBits)
	if err != nil {
		return cfg, err
	}
	cfg.NodeBits = uint8(nodeBits)
	cfg.SeqBits = uint8(seqBits)
	return cfg, nil
}

func generateSQL(cfg Config) string {
	timeShift := cfg.TimeShift()
	maxNode := cfg.MaxNode()
	maxSeq := cfg.MaxSeq()
	nodeMask := cfg.NodeMask()
	seqMask := cfg.SeqMask()

	return fmt.Sprintf(`
-- Sequences
CREATE SEQUENCE IF NOT EXISTS usid_seq CYCLE MAXVALUE %d;
CREATE SEQUENCE IF NOT EXISTS usid_node_seq CYCLE MINVALUE 1 MAXVALUE %d;

-- Get next node ID for app instance (1-%d)
CREATE OR REPLACE FUNCTION usid_next_node()
  RETURNS int
  LANGUAGE sql
  VOLATILE
  AS $$
  SELECT nextval('usid_node_seq')::int;
$$;

-- Generate usid (node 0 for Postgres)
CREATE OR REPLACE FUNCTION usid()
  RETURNS bigint
  LANGUAGE plpgsql
  VOLATILE
  AS $$
DECLARE
  epoch bigint := %d;
  now_us bigint;
  seq bigint;
BEGIN
  now_us := (extract(epoch FROM clock_timestamp()) * 1000000)::bigint - epoch;
  seq := nextval('usid_seq') & %d;
  RETURN (now_us << %d) | (0 << %d) | seq;  -- node 0
END;
$$;

-- Constants
CREATE OR REPLACE FUNCTION omni_usid() RETURNS bigint LANGUAGE sql IMMUTABLE AS $$ SELECT 9223372036854775807::bigint; $$;
CREATE OR REPLACE FUNCTION nil_usid() RETURNS bigint LANGUAGE sql IMMUTABLE AS $$ SELECT 0::bigint; $$;
CREATE OR REPLACE FUNCTION is_omni_usid(id bigint) RETURNS boolean LANGUAGE sql IMMUTABLE AS $$ SELECT id = 9223372036854775807; $$;
CREATE OR REPLACE FUNCTION is_nil_usid(id bigint) RETURNS boolean LANGUAGE sql IMMUTABLE AS $$ SELECT id = 0; $$;

-- Extract components
CREATE OR REPLACE FUNCTION ts_from_usid(id bigint)
  RETURNS timestamp without time zone
  LANGUAGE sql
  IMMUTABLE PARALLEL SAFE STRICT LEAKPROOF
  AS $$
  SELECT to_timestamp(((id >> %d) + %d)::numeric / 1000000);
$$;

CREATE OR REPLACE FUNCTION node_from_usid(id bigint)
  RETURNS int
  LANGUAGE sql
  IMMUTABLE PARALLEL SAFE STRICT LEAKPROOF
  AS $$
  SELECT ((id >> %d) & %d)::int;
$$;

CREATE OR REPLACE FUNCTION seq_from_usid(id bigint)
  RETURNS int
  LANGUAGE sql
  IMMUTABLE PARALLEL SAFE STRICT LEAKPROOF
  AS $$
  SELECT (id & %d)::int;
$$;

-- Base58 encoding/decoding
CREATE OR REPLACE FUNCTION b58_to_usid(encoded_id varchar(11))
  RETURNS bigint
  LANGUAGE plpgsql
  IMMUTABLE PARALLEL SAFE STRICT LEAKPROOF
  AS $$
DECLARE
  alphabet char(58) := '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';
  c char(1);
  p int;
  result bigint := 0;
BEGIN
  FOR i IN 1..char_length(encoded_id) LOOP
    c := substring(encoded_id FROM i FOR 1);
    p := position(c IN alphabet);
    IF p = 0 THEN
      RAISE EXCEPTION 'Invalid base58 character: %%', c;
    END IF;
    result := (result * 58) + (p - 1);
  END LOOP;
  RETURN result;
END;
$$;

CREATE OR REPLACE FUNCTION usid_to_b58(id bigint)
  RETURNS varchar(11)
  LANGUAGE plpgsql
  IMMUTABLE PARALLEL SAFE STRICT LEAKPROOF
  AS $$
DECLARE
  alphabet char(58) := '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';
  result varchar(11) := '';
  remainder int;
BEGIN
  IF id = 0 THEN
    RETURN '1';
  END IF;
  WHILE id > 0 LOOP
    remainder := (id %% 58)::int;
    result := substring(alphabet FROM remainder + 1 FOR 1) || result;
    id := id / 58;
  END LOOP;
  RETURN result;
END;
$$;

-- Base64 encoding/decoding
CREATE OR REPLACE FUNCTION b64_to_usid(encoded_id varchar(12))
  RETURNS bigint
  LANGUAGE sql
  IMMUTABLE PARALLEL SAFE STRICT LEAKPROOF
  AS $$
  SELECT (
    (get_byte(decode(encoded_id, 'base64'), 0)::bigint << 56) |
    (get_byte(decode(encoded_id, 'base64'), 1)::bigint << 48) |
    (get_byte(decode(encoded_id, 'base64'), 2)::bigint << 40) |
    (get_byte(decode(encoded_id, 'base64'), 3)::bigint << 32) |
    (get_byte(decode(encoded_id, 'base64'), 4)::bigint << 24) |
    (get_byte(decode(encoded_id, 'base64'), 5)::bigint << 16) |
    (get_byte(decode(encoded_id, 'base64'), 6)::bigint << 8) |
    (get_byte(decode(encoded_id, 'base64'), 7)::bigint)
  );
$$;

CREATE OR REPLACE FUNCTION usid_to_b64(id bigint)
  RETURNS varchar(12)
  LANGUAGE sql
  IMMUTABLE PARALLEL SAFE STRICT LEAKPROOF
  AS $$
  SELECT encode(
    set_byte(set_byte(set_byte(set_byte(
    set_byte(set_byte(set_byte(set_byte(
      '\x0000000000000000'::bytea,
      0, ((id >> 56) & 255)::int),
      1, ((id >> 48) & 255)::int),
      2, ((id >> 40) & 255)::int),
      3, ((id >> 32) & 255)::int),
      4, ((id >> 24) & 255)::int),
      5, ((id >> 16) & 255)::int),
      6, ((id >> 8) & 255)::int),
      7, (id & 255)::int),
    'base64'
  );
$$;

-- Hex encoding/decoding
CREATE OR REPLACE FUNCTION hex_to_usid(encoded_id text)
  RETURNS bigint
  LANGUAGE sql
  IMMUTABLE PARALLEL SAFE STRICT LEAKPROOF
  AS $$
  SELECT ('x' || lpad(encoded_id, 16, '0'))::bit(64)::bigint;
$$;

CREATE OR REPLACE FUNCTION usid_to_hex(id bigint)
  RETURNS text
  LANGUAGE sql
  IMMUTABLE PARALLEL SAFE STRICT LEAKPROOF
  AS $$
  SELECT to_hex(id);
$$;
`,
		maxSeq,       // usid_seq MAXVALUE
		maxNode,      // usid_node_seq MAXVALUE
		maxNode,      // comment: 1-maxNode
		cfg.Epoch,    // epoch in usid()
		seqMask,      // seq mask in usid()
		timeShift,    // time shift in usid()
		cfg.SeqBits,  // node shift in usid()
		timeShift,    // time shift in ts_from_usid
		cfg.Epoch,    // epoch in ts_from_usid
		cfg.SeqBits,  // node shift in node_from_usid
		nodeMask,     // node mask in node_from_usid
		seqMask,      // seq mask in seq_from_usid
	)
}
