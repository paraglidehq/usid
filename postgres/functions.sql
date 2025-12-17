-- Generate usid (node 0 for Postgres)
CREATE SEQUENCE IF NOT EXISTS usid_seq;

CREATE OR REPLACE FUNCTION usid()
  RETURNS bigint
  LANGUAGE plpgsql
  VOLATILE
  AS $$
DECLARE
  epoch bigint := 1765947799213000;
  now_us bigint;
  seq bigint;
BEGIN
  now_us := (extract(epoch FROM clock_timestamp()) * 1000000)::bigint - epoch;
  seq := nextval('usid_seq') & 255;
  RETURN (now_us << 12) | seq;  -- node 0
END;
$$;

-- Max usid (all bits set except sign)
CREATE OR REPLACE FUNCTION omni_usid()
  RETURNS bigint
  LANGUAGE plpgsql
  IMMUTABLE PARALLEL SAFE STRICT LEAKPROOF
  AS $$
BEGIN
  RETURN 9223372036854775807;  -- max int64
END;
$$;

CREATE OR REPLACE FUNCTION is_omni_usid(id bigint)
  RETURNS boolean
  LANGUAGE plpgsql
  IMMUTABLE PARALLEL SAFE STRICT LEAKPROOF
  AS $$
BEGIN
  RETURN id = 9223372036854775807;
END;
$$;

-- Nil usid (zero)
CREATE OR REPLACE FUNCTION nil_usid()
  RETURNS bigint
  LANGUAGE plpgsql
  IMMUTABLE PARALLEL SAFE STRICT LEAKPROOF
  AS $$
BEGIN
  RETURN 0;
END;
$$;

CREATE OR REPLACE FUNCTION is_nil_usid(id bigint)
  RETURNS boolean
  LANGUAGE plpgsql
  IMMUTABLE PARALLEL SAFE STRICT LEAKPROOF
  AS $$
BEGIN
  RETURN id = 0;
END;
$$;

-- Extract timestamp from usid
CREATE OR REPLACE FUNCTION ts_from_usid(id bigint)
  RETURNS timestamp without time zone
  LANGUAGE sql
  IMMUTABLE PARALLEL SAFE STRICT LEAKPROOF
  AS $$
  SELECT to_timestamp(((id >> 12) + 1765947799213000)::numeric / 1000000);
$$;

-- Extract node from usid
CREATE OR REPLACE FUNCTION node_from_usid(id bigint)
  RETURNS int
  LANGUAGE sql
  IMMUTABLE PARALLEL SAFE STRICT LEAKPROOF
  AS $$
  SELECT ((id >> 8) & 15)::int;
$$;

-- Extract sequence from usid
CREATE OR REPLACE FUNCTION seq_from_usid(id bigint)
  RETURNS int
  LANGUAGE sql
  IMMUTABLE PARALLEL SAFE STRICT LEAKPROOF
  AS $$
  SELECT (id & 255)::int;
$$;

-- Decode base58 to usid
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
      RAISE EXCEPTION 'Invalid base58 character: %', c;
    END IF;
    result := (result * 58) + (p - 1);
  END LOOP;
  RETURN result;
END;
$$;

-- Encode usid to base58
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
    remainder := (id % 58)::int;
    result := substring(alphabet FROM remainder + 1 FOR 1) || result;
    id := id / 58;
  END LOOP;
  RETURN result;
END;
$$;
