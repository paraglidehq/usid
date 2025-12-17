package usid

// DefaultObfuscator, when set, obfuscates all external representations
// (String, Format, JSON, etc.) while keeping internal values raw.
// Set this once at startup before generating or parsing IDs.
var DefaultObfuscator *Obfuscator

// Obfuscator XORs IDs with a key to hide timestamps and sequences
// in external representations.
type Obfuscator struct {
	key int64
}

// NewObfuscator creates an obfuscator with the given key.
// Use a random int64 and keep it secret.
func NewObfuscator(key int64) *Obfuscator {
	return &Obfuscator{key: key}
}

// SetObfuscator sets the DefaultObfuscator with the given key.
// Call once at startup to enable obfuscation.
func SetObfuscator(key int64) {
	DefaultObfuscator = NewObfuscator(key)
}

// Obfuscate XORs the ID with the key.
func (o *Obfuscator) Obfuscate(id ID) ID {
	return ID(int64(id) ^ o.key)
}

// Deobfuscate reverses obfuscation (XOR is its own inverse).
func (o *Obfuscator) Deobfuscate(id ID) ID {
	return ID(int64(id) ^ o.key)
}

// obfuscate applies DefaultObfuscator if set.
func obfuscate(id ID) ID {
	if DefaultObfuscator != nil {
		return DefaultObfuscator.Obfuscate(id)
	}
	return id
}

// deobfuscate reverses obfuscation if DefaultObfuscator is set.
func deobfuscate(id ID) ID {
	if DefaultObfuscator != nil {
		return DefaultObfuscator.Deobfuscate(id)
	}
	return id
}
