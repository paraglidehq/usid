# usid

Time-ordered 64-bit IDs. Half the size of UUIDv7, fits in a `bigint`.

```
UUIDv7:  019234a5-f78b-7c3d-8a1e-3f9b2c8d4e6f  (36 chars, 16 bytes)
usid:    3kTMd92jFk                            (11 chars, 8 bytes)
```

## Installation

```bash
go get github.com/paraglidehq/usid
```

## How it works

```
[1 sign][51 bits µs timestamp][6 bits node][6 bits sequence]
```

**Timestamp** (51 bits): Microseconds since epoch (~71 years). Time-ordered for index-friendly inserts.

**Node ID** (6 bits): Identifies which instance generated the ID. Each instance gets its own "lane"—collisions are impossible as long as node IDs are unique.

**Sequence** (6 bits): Handles multiple IDs within the same microsecond from one instance. You'll never hit this limit in practice.

## Quick start

```go
import "github.com/paraglidehq/usid"

func main() {
    usid.SetNodeID(1)  // Assign once at startup
    
    id := usid.New()
    fmt.Println(id)              // "3kTMd92jFk"
    fmt.Println(id.Timestamp())  // 2025-12-16 12:34:56.789
}
```

## Node ID assignment

Unique node IDs guarantee no collisions—each instance has its own "lane" in the ID space.

Shared node IDs risk collision when two instances generate an ID in the same microsecond. Rough collision rates for two instances sharing a node (assuming uniform distribution):

| IDs/sec per instance | Collision rate |
|----------------------|----------------|
| 10                   | ~1 per 3 hours |
| 100                  | ~1 per 2 minutes |
| 1,000                | ~1 per second |

Real traffic is bursty, so these are optimistic. For N instances sharing a node, multiply by N×(N-1)/2 pairs.

**If collisions are acceptable** (e.g., you retry on unique constraint violation): shared node IDs are fine at low throughput.

**If collisions are unacceptable**: use unique node IDs.

Size your node bits to your max concurrent instances:

| Max instances | NodeBits |
|---------------|----------|
| 15            | 4        |
| 63            | 6 (default) |
| 255           | 8        |

Node 0 is reserved for Postgres (see below), so app instances use 1–63.

### Assignment strategies

```go
// From database sequence (recommended)
node, _ := postgres.NextNode(ctx, db)
usid.SetNodeID(node)

// From environment
usid.SetNodeID(mustParseInt(os.Getenv("NODE_ID")))

// From Kubernetes pod ordinal
// pod-0 → node 1, pod-1 → node 2, etc.
hostname, _ := os.Hostname()
parts := strings.Split(hostname, "-")
ordinal, _ := strconv.ParseInt(parts[len(parts)-1], 10, 64)
usid.SetNodeID((ordinal % 63) + 1)
```

## Postgres

Store as `bigint`:

```sql
CREATE TABLE users (
    id bigint PRIMARY KEY DEFAULT usid(),
    email text NOT NULL
);
```

Run migrations to install Postgres functions:

```go
import "github.com/paraglidehq/usid/postgres"

postgres.Migrate(ctx, db, postgres.DefaultConfig())
```

This gives you:

- `usid()` — generate IDs in Postgres (uses node 0)
- `usid_to_b58(id)` / `b58_to_usid(str)` — encoding
- `ts_from_usid(id)` — extract timestamp
- `usid_next_node()` — get next node ID from sequence

Scanning works automatically:

```go
var user User
db.QueryRow("SELECT id, name FROM users WHERE id = $1", id).Scan(&user.ID, &user.Name)
```

## Why not UUIDv7?

UUIDv7 requires no coordination—any instance can generate IDs independently. The tradeoff is size: 16 bytes vs 8 bytes.

If you're storing millions of rows, that's real savings:

- 47% smaller indexes
- 27% smaller total table size
- Faster range scans

If you only have a few thousand rows or coordination is painful, use UUIDv7.

## Why not Snowflake?

Snowflake uses dedicated ID generation services that app servers call over RPC. That's the right architecture at Twitter's scale, but overkill for most systems.

usid generates in-process: no network hop, no single point of failure, no batching complexity. The tradeoff is you need to assign node IDs at startup.

## Why not nanoid?

nanoid generates random IDs with no coordination required. The tradeoffs:

| | usid | nanoid |
|---|------|--------|
| Storage | 8 bytes (bigint) | 21+ bytes (string) |
| Index writes | Sequential (fast) | Random (fragmented) |
| Comparisons | Integer | String |
| Timestamp | Extractable | None |
| Coordination | Node ID at startup | None |

If you need time ordering or care about database performance at scale, use usid. If you just want short random strings and don't want to think about node IDs, nanoid is simpler.

## API

```go
// Generate
id := usid.New()

// Parse
id, err := usid.Parse("3kTMd92jFk")
id := usid.FromStringOrNil("3kTMd92jFk")

// Format
str := id.String()                    // uses DefaultFormat
str := id.Format(usid.FormatBase58)   // "3kTMd92jFk"
str := id.Format(usid.FormatDecimal)  // "10151254716672"
str := id.Format(usid.FormatHash)     // "93b85ee7100"
str := id.Format(usid.FormatBase64)   // "AAAJO4XucQA="

// Extract components
ts := id.Timestamp()  // time.Time
node := id.Node()     // int64
seq := id.Seq()       // int64

// Raw value
n := id.Int64()
bytes := id.Bytes()
```

## JSON

```go
type User struct {
    ID   usid.ID `json:"id"`
    Name string  `json:"name"`
}
// {"id":"3kTMd92jFk","name":"alice"}

type Record struct {
    ID       usid.ID     `json:"id"`
    ParentID usid.NullID `json:"parent_id"`
}
// {"id":"3kTMd92jFk","parent_id":null}
```

## Customizing bit allocation

```go
// Before any ID generation or migrations:
usid.NodeBits = 8  // 255 instances
usid.SeqBits = 4   // still plenty of headroom

// Then set node ID
usid.SetNodeID(node)

// And migrate with matching config
postgres.Migrate(ctx, db, postgres.Config{
    Epoch:    usid.Epoch,
    NodeBits: usid.NodeBits,
    SeqBits:  usid.SeqBits,
})
```

## Obfuscation

Time-ordered IDs leak creation time. If that's a concern, obfuscate:

```go
// Generate once, store in env/config, keep secret
// head -c 8 /dev/urandom | xxd -p → 0x3a1f9c7b2e4d8a05
usid.SetObfuscator(0x3a1f9c7b2e4d8a05)
```

All external representations (strings, JSON, URLs) get XOR'd with your key. Internal values stay raw—you can still extract timestamps and store as `bigint`.

## Benchmarks

| Operation | ns/op | allocs |
|-----------|------:|:------:|
| New | 36.6 | 0 |
| Parse | 7.7 | 0 |
| String | 25.7 | 1 |

### Postgres (10M rows)

| | usid | UUIDv7 |
|---|------|--------|
| Table size | 498 MB | 574 MB |
| Index size | 214 MB | 402 MB |
| Range scan 1K | 0.125 ms | 0.194 ms |

## License

MIT
