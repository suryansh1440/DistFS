# Step 05 — Coordinator Core: Chunking, Reed-Solomon & Checksums

## Goal
Build the three core processing modules in the coordinator: file chunking, Reed-Solomon erasure coding, and SHA-256 checksum calculation.

---

## Architecture Position

```
REST API (Step 06)
    │
    ▼
┌────────────────────────────────┐
│         COORDINATOR CORE       │  ← THIS STEP
│                                │
│  ┌──────────┐  ┌───────────┐  │
│  │ checksum │  │  chunker  │  │
│  │          │  │           │  │
│  │ SHA-256  │  │ Split     │  │
│  │ File     │  │ Merge     │  │
│  │ Shard    │  │ 4MB chunks│  │
│  └──────────┘  └───────────┘  │
│                                │
│  ┌───────────────────────────┐ │
│  │        erasure            │ │
│  │                           │ │
│  │ Encode: chunk → 3D + 1P  │ │
│  │ Decode: reconstruct      │ │
│  │ klauspost/reedsolomon     │ │
│  └───────────────────────────┘ │
│                                │
│  ┌───────────────────────────┐ │
│  │        models             │ │
│  │                           │ │
│  │ File, Chunk, Shard, Node  │ │
│  └───────────────────────────┘ │
└────────────────────────────────┘
    │
    ▼
gRPC → Storage Nodes (Step 04)
PostgreSQL (Step 03)
```

---

## Files to Create

### `coordinator/internal/models/models.go`

```go
package models

import "time"

// File represents a stored file's metadata.
type File struct {
    ID               string    `json:"id"`
    Filename         string    `json:"filename"`
    FileSize         int64     `json:"file_size"`
    OriginalChecksum string    `json:"original_checksum"`
    TotalChunks      int       `json:"total_chunks"`
    DataShards       int       `json:"data_shards"`
    ParityShards     int       `json:"parity_shards"`
    ChunkSize        int       `json:"chunk_size"`
    Status           string    `json:"status"` // UPLOADING, HEALTHY, DEGRADED, FAILED
    CreatedAt        time.Time `json:"created_at"`
    UpdatedAt        time.Time `json:"updated_at"`
}

// Chunk represents a chunk of a file.
type Chunk struct {
    ID         string    `json:"id"`
    FileID     string    `json:"file_id"`
    ChunkIndex int       `json:"chunk_index"`
    ChunkSize  int64     `json:"chunk_size"`
    CreatedAt  time.Time `json:"created_at"`
}

// Shard represents one shard (data or parity) of a chunk.
type Shard struct {
    ID         string    `json:"id"`
    ChunkID    string    `json:"chunk_id"`
    FileID     string    `json:"file_id"`
    ShardIndex int       `json:"shard_index"`
    ShardType  string    `json:"shard_type"` // DATA or PARITY
    NodeID     string    `json:"node_id"`
    ShardSize  int64     `json:"shard_size"`
    Checksum   string    `json:"checksum"`
    Status     string    `json:"status"` // STORED, MISSING, CORRUPTED
    CreatedAt  time.Time `json:"created_at"`
}

// Node represents a storage node in the cluster.
type Node struct {
    ID            string    `json:"id"`
    Name          string    `json:"name"`
    GRPCAddress   string    `json:"grpc_address"`
    Status        string    `json:"status"` // ONLINE, OFFLINE
    TotalStorage  int64     `json:"total_storage"`
    UsedStorage   int64     `json:"used_storage"`
    ShardCount    int       `json:"shard_count"`
    LastHeartbeat time.Time `json:"last_heartbeat"`
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
}

// UploadResult is returned after a successful file upload.
type UploadResult struct {
    FileID   string `json:"file_id"`
    Filename string `json:"filename"`
    FileSize int64  `json:"file_size"`
    Checksum string `json:"checksum"`
    Chunks   int    `json:"chunks"`
    Shards   int    `json:"shards"`
    Status   string `json:"status"`
}

// DownloadResult holds the reconstructed file data.
type DownloadResult struct {
    Filename string `json:"filename"`
    Data     []byte `json:"-"`
    Checksum string `json:"checksum"`
    Valid    bool   `json:"valid"`
    Reconstructed bool `json:"reconstructed"` // true if any shard was missing and recovered
}
```

---

### `coordinator/internal/checksum/checksum.go`

```go
package checksum

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "os"
)

// CalculateFromBytes computes SHA-256 of a byte slice.
func CalculateFromBytes(data []byte) string {
    hash := sha256.Sum256(data)
    return hex.EncodeToString(hash[:])
}

// CalculateFromFile computes SHA-256 of a file by streaming (memory-efficient).
func CalculateFromFile(filePath string) (string, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return "", fmt.Errorf("failed to open file: %w", err)
    }
    defer f.Close()

    hasher := sha256.New()
    if _, err := io.Copy(hasher, f); err != nil {
        return "", fmt.Errorf("failed to hash file: %w", err)
    }

    return hex.EncodeToString(hasher.Sum(nil)), nil
}

// CalculateFromReader computes SHA-256 from an io.Reader (streaming).
func CalculateFromReader(r io.Reader) (string, error) {
    hasher := sha256.New()
    if _, err := io.Copy(hasher, r); err != nil {
        return "", fmt.Errorf("failed to hash: %w", err)
    }
    return hex.EncodeToString(hasher.Sum(nil)), nil
}

// Verify checks if data matches an expected checksum.
func Verify(data []byte, expectedChecksum string) bool {
    actual := CalculateFromBytes(data)
    return actual == expectedChecksum
}
```

---

### `coordinator/internal/chunker/chunker.go`

```go
package chunker

import (
    "fmt"
    "math"
)

const DefaultChunkSize = 4 * 1024 * 1024 // 4MB

// ChunkResult holds one chunk's data and its index.
type ChunkResult struct {
    Index int
    Data  []byte
    Size  int64
}

// SplitData divides raw file bytes into fixed-size chunks.
// The last chunk may be smaller than chunkSize.
func SplitData(data []byte, chunkSize int) ([]ChunkResult, error) {
    if len(data) == 0 {
        return nil, fmt.Errorf("cannot chunk empty data")
    }
    if chunkSize <= 0 {
        return nil, fmt.Errorf("chunk size must be positive")
    }

    totalChunks := int(math.Ceil(float64(len(data)) / float64(chunkSize)))
    chunks := make([]ChunkResult, 0, totalChunks)

    for i := 0; i < len(data); i += chunkSize {
        end := i + chunkSize
        if end > len(data) {
            end = len(data)
        }

        chunk := make([]byte, end-i)
        copy(chunk, data[i:end])

        chunks = append(chunks, ChunkResult{
            Index: len(chunks),
            Data:  chunk,
            Size:  int64(len(chunk)),
        })
    }

    return chunks, nil
}

// MergeChunks reassembles chunks back into the original file data.
// Chunks must be in order by index.
func MergeChunks(chunks []ChunkResult) []byte {
    totalSize := int64(0)
    for _, c := range chunks {
        totalSize += c.Size
    }

    result := make([]byte, 0, totalSize)
    for _, c := range chunks {
        result = append(result, c.Data...)
    }

    return result
}

// CalculateTotalChunks returns how many chunks a file of the given size will produce.
func CalculateTotalChunks(fileSize int64, chunkSize int) int {
    return int(math.Ceil(float64(fileSize) / float64(chunkSize)))
}
```

---

### `coordinator/internal/erasure/erasure.go`

```go
package erasure

import (
    "fmt"
    "log"

    "github.com/klauspost/reedsolomon"
)

// Config holds Reed-Solomon configuration.
type Config struct {
    DataShards   int // K — number of data shards
    ParityShards int // M — number of parity shards
}

// DefaultConfig returns 3 data + 1 parity (fits 4 nodes).
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
//
// Input:  raw chunk bytes (e.g., 4MB)
// Output: slice of (K+M) shards, each shard is []byte
//
// The data shards concatenated together reproduce the original chunk.
// The parity shards enable reconstruction if any shard(s) are missing.
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

    // Verify that encoding is correct
    ok, err := e.enc.Verify(shards)
    if err != nil {
        return nil, fmt.Errorf("failed to verify shards: %w", err)
    }
    if !ok {
        return nil, fmt.Errorf("shard verification failed after encoding")
    }

    log.Printf("RS Encode: chunk=%d bytes → %d data + %d parity shards (each ~%d bytes)",
        len(chunkData), e.config.DataShards, e.config.ParityShards, len(shards[0]))

    return shards, nil
}

// Decode takes a set of shards (some may be nil/missing) and reconstructs the original chunk.
//
// Input:  slice of (K+M) shards — missing shards should be nil
// Output: the original chunk data
//
// Requires at least K non-nil shards to succeed.
func (e *Encoder) Decode(shards [][]byte, originalChunkSize int64) ([]byte, error) {
    // Count available shards
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

    // Reconstruct missing shards
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

        log.Printf("RS Decode: ✓ Successfully reconstructed %d missing shard(s)", missing)
    }

    // Join data shards back into the original chunk
    // Only join the first K shards (data shards), not the parity shards
    totalSize := 0
    for i := 0; i < e.config.DataShards; i++ {
        totalSize += len(shards[i])
    }

    result := make([]byte, 0, totalSize)
    for i := 0; i < e.config.DataShards; i++ {
        result = append(result, shards[i]...)
    }

    // Trim to original chunk size (the library may have padded the last shard)
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
```

---

## How Encoding Works (Detailed Example)

```
Original chunk: 4,194,304 bytes (4MB)

                ┌──────────────────────┐
                │    4MB Chunk Data    │
                └──────────┬───────────┘
                           │
                    reedsolomon.Split()
                           │
           ┌───────────────┼───────────────┐
           │               │               │
    ┌──────▼──────┐ ┌──────▼──────┐ ┌──────▼──────┐
    │ Data Shard 0│ │ Data Shard 1│ │ Data Shard 2│
    │  1,398,102  │ │  1,398,102  │ │  1,398,100  │
    │    bytes    │ │    bytes    │ │    bytes    │
    └─────────────┘ └─────────────┘ └─────────────┘
           │               │               │
           └───────────────┼───────────────┘
                           │
                    reedsolomon.Encode()
                           │
                    ┌──────▼──────┐
                    │Parity Shard │
                    │  1,398,102  │
                    │    bytes    │
                    └─────────────┘
```

Each shard is approximately 1/3 of the original chunk size (plus padding).

---

## How Decoding Works (Failure Case)

```
Node 2 is OFFLINE → Data Shard 1 is MISSING

Available:
    Data Shard 0  ✓  (from Node 1)
    Data Shard 1  ✗  MISSING (Node 2 offline)
    Data Shard 2  ✓  (from Node 3)
    Parity Shard  ✓  (from Node 4)

shards = [
    []byte{...},     // shard 0 — available
    nil,              // shard 1 — MISSING
    []byte{...},     // shard 2 — available
    []byte{...},     // parity  — available
]

        ↓ reedsolomon.Reconstruct()

shards = [
    []byte{...},     // shard 0 — available
    []byte{...},     // shard 1 — RECONSTRUCTED ✓
    []byte{...},     // shard 2 — available
    []byte{...},     // parity  — available
]

        ↓ Join data shards 0, 1, 2

Original 4MB chunk restored ✓
```

---

## Verification

```bash
# Build the coordinator to make sure everything compiles
cd coordinator
go build ./...

# Unit test idea (you can add this in Step 11):
# 1. Create 4MB random data
# 2. Encode → get 4 shards
# 3. Set one shard to nil
# 4. Decode → should recover original data
# 5. Compare checksums
```

---

## What This Step Achieves
- **Checksum module**: SHA-256 for files, shards, and verification
- **Chunker module**: Split files into 4MB chunks, merge them back
- **Erasure module**: Reed-Solomon encode/decode with 3+1 configuration
- All three are pure functions — no I/O dependencies — easy to test
- Ready for the REST API (Step 06) and reconstruction (Step 08)
