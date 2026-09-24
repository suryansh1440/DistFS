// Package checksum provides SHA-256 hashing for file and shard integrity.
// Used at two levels:
//   - File-level:  hash the complete original file (stored in PostgreSQL)
//   - Shard-level: hash each individual shard (verified on store and retrieve)
package checksum

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// CalculateFromBytes computes the SHA-256 hex digest of a byte slice.
func CalculateFromBytes(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// CalculateFromFile streams a file from disk and returns its SHA-256 hex digest.
// Memory-efficient: reads in chunks instead of loading the entire file.
func CalculateFromFile(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for hashing: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// CalculateFromReader computes SHA-256 from any io.Reader (streaming).
func CalculateFromReader(r io.Reader) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, r); err != nil {
		return "", fmt.Errorf("failed to hash from reader: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// Verify checks if the SHA-256 of data matches the expected checksum.
// Returns true if they match — used for failsafe validation.
func Verify(data []byte, expectedChecksum string) bool {
	return CalculateFromBytes(data) == expectedChecksum
}
