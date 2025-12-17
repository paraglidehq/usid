# usid

Time-ordered 64-bit IDs with microsecond precision.

## Why?

UUIDv7 is great but 128 bit universally unique IDs are overkill for most applications:

```
UUIDv7:  019234a5-f78b-7c3d-8a1e-3f9b2c8d4e6f  (36 chars, 16 bytes)
usid:    3kTMd92jFk                            (11 chars, 8 bytes)
```

Half the storage, one-third the display length. Fits in a `bigint`. Looks better in URLs.

Like Snowflake, usid uses a time-ordered layout for index-friendly inserts. Unlike Snowflake, it uses microsecond precision—so you can trade node bits for throughput depending on your needs.

## Layout

```
[1 sign][51 bits µs timestamp][4 bits node][8 bits sequence]
```

- **51 bits**: Microseconds since epoch (~71 years)
- **4 bits**: Node ID (0-15)
- **8 bits**: Sequence counter (256 IDs/µs/node)

## Installation

```bash
go get github.com/paraglidehq/usid
```

## Usage

```go
import "github.com/paraglidehq/usid"

// Generate IDs
id := usid.New()

// String encoding (uses DefaultFormat)
str := id.String()  // "3kTMd92jFk"

// Parse back (uses DefaultFormat)
id, err := usid.Parse(str)
id = usid.FromStringOrNil(str)

// Explicit format parsing
id, err = usid.ParseBase58("3kTMd92jFk")
id, err = usid.ParseHash("93b85ee7100")
id, err = usid.ParseBase64("AAAJO4XucQA=")
id, err = usid.ParseDecimal("10151254716672")

// Binary
bytes := id.Bytes()              // []byte (8 bytes, big-endian)
id, err = usid.FromBytes(bytes)

// Extract components
ts := id.Timestamp()  // time.Time
node := id.Node()     // int64
seq := id.Seq()       // int64

// Get raw int64
n := id.Int64()
```

For multi-node deployments, set the node ID at startup:

```go
usid.SetNodeID(2)  // 1-15 for app nodes (0 reserved for Postgres)
```

Or manage generators manually:

```go
gen := usid.NewGenerator(0)
id := gen.Generate()
```

## Formatting

```go
// Set default format (affects String(), Parse(), JSON, etc.)
usid.DefaultFormat = usid.FormatBase58  // default

// All formats
id.Format(usid.FormatBase58)   // "5bf4cqs5"
id.Format(usid.FormatDecimal)  // "10151254716672"
id.Format(usid.FormatHash)     // "93b85ee7100" (hex, no leading zeros)
id.Format(usid.FormatBase64)   // "AAAJO4XucQA="
```

## JSON

```go
usid.DefaultFormat = usid.FormatBase58

type User struct {
    ID   usid.ID `json:"id"`
    Name string  `json:"name"`
}

// Marshals to: {"id":"3kTMd92jFk","name":"alice"}
```

For nullable fields:

```go
type Record struct {
    ID       usid.ID     `json:"id"`
    ParentID usid.NullID `json:"parent_id"`
}

// Marshals to: {"id":"3kTMd92jFk","parent_id":null}
```

## Database

Store as `bigint` in Postgres:

```sql
CREATE TABLE users (
    id bigint PRIMARY KEY,
    email text NOT NULL
);
```

Or generate IDs in Postgres (uses node 0):

```sql
CREATE SEQUENCE usid_seq;

CREATE FUNCTION usid_generate() RETURNS bigint AS $$
DECLARE
    epoch bigint := 1765947799213000;
    now_us bigint;
    seq bigint;
BEGIN
    now_us := (extract(epoch from clock_timestamp()) * 1000000)::bigint - epoch;
    seq := nextval('usid_seq') & 255;
    RETURN (now_us << 12) | seq;  -- node 0
END;
$$ LANGUAGE plpgsql;

CREATE TABLE users (
    id bigint PRIMARY KEY DEFAULT usid_generate(),
    email text NOT NULL
);
```

Scanning works automatically:

```go
var user User
db.QueryRow("SELECT id, name FROM users WHERE id = $1", id).Scan(&user.ID, &user.Name)

// For nullable columns
var record struct {
    ID       usid.ID
    ParentID usid.NullID
}
db.QueryRow("SELECT id, parent_id FROM records WHERE id = $1", id).Scan(&record.ID, &record.ParentID)
```

No special types, no extensions. Works with any database that supports 64-bit integers.

## Node ID Assignment

Node 0 is reserved for Postgres. App instances use nodes 1-15 (default config).

```go
// Environment variable
nodeID, _ := strconv.ParseInt(os.Getenv("NODE_ID"), 10, 64)
usid.SetNodeID(nodeID)

// Kubernetes pod ordinal (pod-0 gets node 1, pod-1 gets node 2, etc.)
hostname, _ := os.Hostname()  // "app-0"
parts := strings.Split(hostname, "-")
ordinal, _ := strconv.ParseInt(parts[len(parts)-1], 10, 64)
usid.SetNodeID((ordinal % 15) + 1)

// Database sequence (once at startup)
var nodeID int64
db.QueryRow("SELECT (nextval('node_id_seq') % 15) + 1").Scan(&nodeID)
usid.SetNodeID(nodeID)
```

## Comparison

| | usid | Snowflake | UUIDv7 |
|---|------|-----------|--------|
| Size | 8 bytes | 8 bytes | 16 bytes |
| Display (base58) | 11 chars | 11 chars | 22 chars |
| Time precision | µs | ms | ms |
| Throughput/node | 256K/ms | 4K/ms | ∞ |
| Max nodes | 16 (15 app + 1 Postgres) | 1,024 | ∞ |
| Coordination | Node ID | Node ID | None |
| Postgres type | `bigint` | `bigint` | `uuid` |

## Tuning

Need more nodes? Adjust the bit allocation:

```go
// 64 nodes, 64K IDs/ms/node
usid.NodeBits = 6
usid.SeqBits = 6

// 256 nodes, 16K IDs/ms/node  
usid.NodeBits = 8
usid.SeqBits = 4
```

At 10 node bits and 2 seq bits, you match Snowflake exactly—1,024 nodes at 4K/ms/node.

## License

MIT

## Benchmarks

### Go

| Operation | usid | UUIDv7 | Δ |
|-----------|------|--------|---|
| Generate | 63.6 ns/op | 195.2 ns/op | **3.1× faster** |
| String (base58) | 31.8 ns/op | 398.7 ns/op | **12.5× faster** |
| Parse (base58) | 79.5 ns/op | 102.3 ns/op | 1.3× faster |

### Postgres (10M rows)

| | usid | uuid | Δ |
|---|------|------|---|
| Insert | 26.2s | 49.3s | **1.9× faster** |
| Table size | 498 MB | 574 MB | 13% smaller |
| Index size | 214 MB | 402 MB | **47% smaller** |
| Total size | 712 MB | 977 MB | **27% smaller** |
| Range scan 1K | 0.30 ms | 0.93 ms | **3.1× faster** |
