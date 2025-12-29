// Package util provides utility functions for VT-UOS.
package util

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// IDGenerator provides thread-safe UUIDv7 generation with monotonic timestamps.
type IDGenerator struct {
	mu       sync.Mutex
	lastTime int64
	counter  uint16
}

// NewIDGenerator creates a new ID generator.
func NewIDGenerator() *IDGenerator {
	return &IDGenerator{}
}

// NewID generates a new UUIDv7 identifier from this generator.
func (g *IDGenerator) NewID() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now().UnixMilli()

	if now == g.lastTime {
		g.counter++
		if g.counter == 0 {
			for now == g.lastTime {
				time.Sleep(time.Microsecond * 100)
				now = time.Now().UnixMilli()
			}
			g.counter = 0
		}
	} else {
		g.lastTime = now
		g.counter = 0
	}

	return generateUUIDv7(now, g.counter)
}

var generator = &IDGenerator{}

// NewID generates a new UUIDv7 identifier.
// UUIDv7 provides time-ordered identifiers for better database index locality.
func NewID() string {
	generator.mu.Lock()
	defer generator.mu.Unlock()

	// Get current Unix milliseconds
	now := time.Now().UnixMilli()

	// Handle same-millisecond IDs with counter
	if now == generator.lastTime {
		generator.counter++
		if generator.counter == 0 {
			// Counter overflow, wait for next millisecond
			for now == generator.lastTime {
				time.Sleep(time.Microsecond * 100)
				now = time.Now().UnixMilli()
			}
			generator.counter = 0
		}
	} else {
		generator.lastTime = now
		generator.counter = 0
	}

	return generateUUIDv7(now, generator.counter)
}

// generateUUIDv7 creates a UUIDv7 from a timestamp and counter.
func generateUUIDv7(unixMilli int64, counter uint16) string {
	var id [16]byte

	// First 48 bits: Unix timestamp in milliseconds (big endian)
	binary.BigEndian.PutUint32(id[0:4], uint32(unixMilli>>16))
	binary.BigEndian.PutUint16(id[4:6], uint16(unixMilli))

	// Version (4 bits) + random (12 bits)
	// Set version to 7
	id[6] = 0x70 | (byte(counter>>8) & 0x0F)
	id[7] = byte(counter)

	// Variant (2 bits) + random (62 bits)
	var randomBytes [8]byte
	rand.Read(randomBytes[:])
	copy(id[8:], randomBytes[:])
	id[8] = (id[8] & 0x3F) | 0x80 // Set variant to RFC 4122

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(id[0:4]),
		binary.BigEndian.Uint16(id[4:6]),
		binary.BigEndian.Uint16(id[6:8]),
		binary.BigEndian.Uint16(id[8:10]),
		id[10:16],
	)
}

// NewUUID generates a standard UUIDv4 (random) identifier.
// Use NewID() for most cases; this is for compatibility.
func NewUUID() string {
	return uuid.New().String()
}

// ParseID validates and parses a UUID string.
func ParseID(s string) (string, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return "", fmt.Errorf("invalid ID format: %w", err)
	}
	return id.String(), nil
}

// IsValidID checks if a string is a valid UUID format.
func IsValidID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// RegistryNumber generates a vault registry number.
// Format: V{vault_number}-{5-digit sequence}
// Example: V076-00001
type RegistryNumberGenerator struct {
	mu          sync.Mutex
	vaultNumber int
	lastSeq     int
}

// NewRegistryNumberGenerator creates a new registry number generator.
func NewRegistryNumberGenerator(vaultNumber int) *RegistryNumberGenerator {
	return &RegistryNumberGenerator{
		vaultNumber: vaultNumber,
	}
}

// SetLastSequence sets the last used sequence number.
// Call this after loading the highest existing registry number from the database.
func (r *RegistryNumberGenerator) SetLastSequence(seq int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastSeq = seq
}

// Next generates the next registry number.
func (r *RegistryNumberGenerator) Next() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastSeq++
	return fmt.Sprintf("V%03d-%05d", r.vaultNumber, r.lastSeq)
}

// Parse extracts the vault number and sequence from a registry number.
func ParseRegistryNumber(regNum string) (vaultNumber, sequence int, err error) {
	_, err = fmt.Sscanf(regNum, "V%03d-%05d", &vaultNumber, &sequence)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid registry number format: %w", err)
	}
	return vaultNumber, sequence, nil
}

// DeterministicID generates a deterministic ID for testing purposes.
// DO NOT use in production - use NewID() instead.
func DeterministicID(seed int64) string {
	var id [16]byte

	binary.BigEndian.PutUint64(id[0:8], uint64(seed))
	binary.BigEndian.PutUint64(id[8:16], uint64(seed*31))

	// Set version 4 and variant
	id[6] = (id[6] & 0x0F) | 0x40
	id[8] = (id[8] & 0x3F) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(id[0:4]),
		binary.BigEndian.Uint16(id[4:6]),
		binary.BigEndian.Uint16(id[6:8]),
		binary.BigEndian.Uint16(id[8:10]),
		id[10:16],
	)
}
