package erasure

import (
	"fmt"
	"log"

	"github.com/klauspost/reedsolomon"
)

// Config holds Reed-Solomon configuration parameters.
type Config struct {
	DataShards   int // K — number of data shards
	ParityShards int // M — number of parity shards
}

// DefaultConfig returns 3 data + 1 parity (fits 4 nodes cluster).
func DefaultConfig() Config {
	return Config{
		DataShards:   3,
		ParityShards: 1,
	}
}

// Encoder wraps the klauspost/reedsolomon encoder.
type Encoder struct {
	config Config
	enc    reedsolomon.Encoder
}

// NewEncoder creates a new Reed-Solomon encoder with the given config.
func NewEncoder(config Config) (*Encoder, error) {
	enc, err := reedsolomon.New(config.DataShards, config.ParityShards)
	if err != nil {
		return nil, fmt.Errorf("failed to create RS encoder: %w", err)
	}

	return &Encoder{
		config: config,
		enc:    enc,
	}, nil
}

// Encode takes a chunk of data and produces K data shards + M parity shards.
// Input:  raw chunk bytes (e.g. up to 4MB)
// Output: slice of (K+M) shards, each shard is []byte
func (e *Encoder) Encode(chunkData []byte) ([][]byte, error) {
	// Split the chunk into K data shards (the library pads the last shard if needed)
	shards, err := e.enc.Split(chunkData)
	if err != nil {
		return nil, fmt.Errorf("failed to split data into shards: %w", err)
	}

	// Calculate parity shards and fill them into shards[K..K+M-1]
	err = e.enc.Encode(shards)
	if err != nil {
		return nil, fmt.Errorf("failed to encode parity shards: %w", err)
	}

	// Verify that encoding is mathematically sound
	ok, err := e.enc.Verify(shards)
	if err != nil {
		return nil, fmt.Errorf("failed to verify shards: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("shard verification failed after encoding")
	}

	log.Printf("RS Encode: chunk=%d bytes -> %d data + %d parity shards (each ~%d bytes)",
		len(chunkData), e.config.DataShards, e.config.ParityShards, len(shards[0]))

	return shards, nil
}

// Decode takes a set of shards (missing or corrupted shards are nil) and reconstructs the chunk.
// Requires at least K non-nil shards to succeed.
func (e *Encoder) Decode(shards [][]byte, originalChunkSize int64) ([]byte, error) {
	available := 0
	missing := 0
	for i, s := range shards {
		if s != nil {
			available++
		} else {
			missing++
			log.Printf("RS Decode: shard %d is MISSING", i)
		}
	}

	log.Printf("RS Decode: %d available, %d missing (need %d)",
		available, missing, e.config.DataShards)

	if available < e.config.DataShards {
		return nil, fmt.Errorf(
			"not enough shards to reconstruct: have %d, need %d",
			available, e.config.DataShards,
		)
	}

	// Reconstruct missing shards if any are missing
	if missing > 0 {
		err := e.enc.Reconstruct(shards)
		if err != nil {
			return nil, fmt.Errorf("failed to reconstruct missing shards: %w", err)
		}

		// Verify reconstruction
		ok, err := e.enc.Verify(shards)
		if err != nil {
			return nil, fmt.Errorf("failed to verify after reconstruction: %w", err)
		}
		if !ok {
			return nil, fmt.Errorf("verification failed after reconstruction")
		}

		log.Printf("RS Decode: [OK] Successfully reconstructed %d missing shard(s)", missing)
	}

	// Join data shards back into the original chunk (first K shards only)
	totalSize := 0
	for i := 0; i < e.config.DataShards; i++ {
		totalSize += len(shards[i])
	}

	result := make([]byte, 0, totalSize)
	for i := 0; i < e.config.DataShards; i++ {
		result = append(result, shards[i]...)
	}

	// Trim to original chunk size (discard padding added during Split)
	if int64(len(result)) > originalChunkSize {
		result = result[:originalChunkSize]
	}

	return result, nil
}

// TotalShards returns K + M.
func (e *Encoder) TotalShards() int {
	return e.config.DataShards + e.config.ParityShards
}

// DataShards returns K.
func (e *Encoder) DataShards() int {
	return e.config.DataShards
}

// ParityShards returns M.
func (e *Encoder) ParityShards() int {
	return e.config.ParityShards
}
