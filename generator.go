package usid

import "time"

// Configuration variables for USID generation.
// Modify these before generating any IDs if you need custom bit layouts.
var (
	// Epoch is the custom epoch in microseconds (default: 2025-12-16).
	// IDs store time as microseconds since this epoch.
	Epoch int64 = 1765947799213000

	// NodeBits is the number of bits allocated for the node ID (default: 6, max 64 nodes).
	NodeBits uint8 = 6

	// SeqBits is the number of bits allocated for the sequence number (default: 6, max 64 per µs).
	SeqBits uint8 = 6

	// DefaultFormat is the default string encoding format for IDs.
	DefaultFormat Format = FormatBase58
)

// DefaultGenerator is used by New(). Set via SetNodeID().
var DefaultGenerator = NewGenerator(1)

// SetNodeID initializes the DefaultGenerator with the given node ID.
// Call this once at startup before using New().
func SetNodeID(node int64) {
	DefaultGenerator = NewGenerator(node)
}

// New generates an ID using the DefaultGenerator.
// Panics if SetNodeID() hasn't been called.
func New() ID {
	if DefaultGenerator == nil {
		panic("usid: call SetNodeID() before using New()")
	}
	return DefaultGenerator.Generate()
}

// NewGenerator creates a Generator for the given node ID.
// The node ID must be in the range [0, 2^NodeBits - 1].
// Panics if node is out of range.
func NewGenerator(node int64) *Generator {
	nodeMax := int64((1 << NodeBits) - 1)
	if node < 0 || node > nodeMax {
		panic("usid: node ID out of range")
	}
	return &Generator{
		node:      node,
		seqMask:   (1 << SeqBits) - 1,
		nodeShift: SeqBits,
		timeShift: SeqBits + NodeBits,
	}
}

// Generate produces a new unique ID.
// Safe for concurrent use.
func (g *Generator) Generate() ID {
	for {
		now := time.Now().UnixMicro() - Epoch

		old := g.state.Load()
		oldTime := int64(old >> SeqBits)
		oldSeq := int64(old & uint64(g.seqMask))

		var newTime, seq int64
		if now > oldTime {
			// Time moved forward, reset sequence
			newTime = now
			seq = 0
		} else {
			// Time is same or went backward, increment sequence
			seq = oldSeq + 1
			if seq > g.seqMask {
				// Sequence exhausted, spin until time advances
				continue
			}
			newTime = oldTime
		}

		if g.state.CompareAndSwap(old, uint64(newTime<<SeqBits)|uint64(seq)) {
			return ID((newTime << g.timeShift) | (g.node << g.nodeShift) | seq)
		}
	}
}

// Deprecated: Use ID.Timestamp() instead
func Timestamp(id int64) time.Time {
	return ID(id).Timestamp()
}

// Deprecated: Use ID.Node() instead
func Node(id int64) int64 {
	return ID(id).Node()
}

// Deprecated: Use ID.Seq() instead
func Seq(id int64) int64 {
	return ID(id).Seq()
}
