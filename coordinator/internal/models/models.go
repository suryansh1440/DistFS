// Package models defines all shared data structures used across the coordinator.
// These structs map directly to PostgreSQL tables and API responses.
package models

import "time"

// ═══════════════════════════════════════════════════════════════
// Database Models — one struct per PostgreSQL table
// ═══════════════════════════════════════════════════════════════

// File represents a stored file's metadata in PostgreSQL.
// The actual bytes are split into chunks → shards → distributed to storage nodes.
type File struct {
	ID               string    `json:"id"`
	Filename         string    `json:"filename"`
	FileSize         int64     `json:"file_size"`
	OriginalChecksum string    `json:"original_checksum"` // SHA-256 of the complete original file
	TotalChunks      int       `json:"total_chunks"`
	DataShards       int       `json:"data_shards"`  // K (Reed-Solomon)
	ParityShards     int       `json:"parity_shards"` // M (Reed-Solomon)
	ChunkSize        int       `json:"chunk_size"`    // bytes per chunk (default 4MB)
	Status           string    `json:"status"`        // UPLOADING | HEALTHY | DEGRADED | FAILED
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Chunk represents one chunk of a file.
// Each chunk is independently Reed-Solomon encoded into K+M shards.
type Chunk struct {
	ID         string    `json:"id"`
	FileID     string    `json:"file_id"`
	ChunkIndex int       `json:"chunk_index"` // 0-based position within the file
	ChunkSize  int64     `json:"chunk_size"`  // actual size (last chunk may be smaller)
	CreatedAt  time.Time `json:"created_at"`
}

// Shard represents one shard (data or parity) of a chunk.
// Metadata only — actual shard bytes live on the storage node's disk.
type Shard struct {
	ID         string    `json:"id"`
	ChunkID    string    `json:"chunk_id"`
	FileID     string    `json:"file_id"`
	ShardIndex int       `json:"shard_index"` // 0..K-1 = data, K..K+M-1 = parity
	ShardType  string    `json:"shard_type"`  // DATA | PARITY
	NodeID     string    `json:"node_id"`     // which storage node holds this shard
	ShardSize  int64     `json:"shard_size"`
	Checksum   string    `json:"checksum"` // SHA-256 of this individual shard
	Status     string    `json:"status"`   // STORED | MISSING | CORRUPTED
	CreatedAt  time.Time `json:"created_at"`
}

// Node represents a storage node in the cluster.
type Node struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	GRPCAddress   string    `json:"grpc_address"`
	Status        string    `json:"status"` // ONLINE | OFFLINE
	TotalStorage  int64     `json:"total_storage"`
	UsedStorage   int64     `json:"used_storage"`
	ShardCount    int       `json:"shard_count"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ═══════════════════════════════════════════════════════════════
// API Response Models
// ═══════════════════════════════════════════════════════════════

// UploadResult is the API response after a successful file upload.
type UploadResult struct {
	FileID   string `json:"file_id"`
	Filename string `json:"filename"`
	FileSize int64  `json:"file_size"`
	Checksum string `json:"checksum"`
	Chunks   int    `json:"chunks"`
	Shards   int    `json:"shards"`
	Status   string `json:"status"`
}

// DownloadResult holds the reconstructed file data for download.
type DownloadResult struct {
	Filename        string `json:"filename"`
	Data            []byte `json:"-"` // not serialized — sent as raw bytes
	Checksum        string `json:"checksum"`
	Valid           bool   `json:"valid"`
	Reconstructed   bool   `json:"reconstructed"`    // true if RS recovery was needed
	MissingShards   int    `json:"missing_shards"`
	RecoveredShards int    `json:"recovered_shards"`
}
