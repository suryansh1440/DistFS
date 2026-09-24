package reconstruction

import (
	"fmt"
	"log"
	"sort"

	"github.com/surya/dist-file-storage/coordinator/internal/checksum"
	"github.com/surya/dist-file-storage/coordinator/internal/chunker"
	"github.com/surya/dist-file-storage/coordinator/internal/db"
	"github.com/surya/dist-file-storage/coordinator/internal/erasure"
	"github.com/surya/dist-file-storage/coordinator/internal/models"
	"github.com/surya/dist-file-storage/coordinator/internal/shardmanager"
)

// ReconstructionResult holds the result of a file download and reconstruction attempt.
type ReconstructionResult struct {
	FileData        []byte
	Valid           bool
	Reconstructed   bool   // true if any missing or corrupted shard had to be recovered
	Checksum        string // SHA-256 of the reconstructed file
	MissingShards   int
	RecoveredShards int
	Error           error
}

// Reconstructor handles file reconstruction from distributed shards.
type Reconstructor struct {
	shardMgr *shardmanager.ShardManager
	encoder  *erasure.Encoder
}

func NewReconstructor(sm *shardmanager.ShardManager, enc *erasure.Encoder) *Reconstructor {
	return &Reconstructor{
		shardMgr: sm,
		encoder:  enc,
	}
}

// ReconstructFile fetches all shards, reconstructs missing ones, and rebuilds the complete file.
func (r *Reconstructor) ReconstructFile(file *models.File) *ReconstructionResult {
	result := &ReconstructionResult{}

	log.Printf("=== Reconstructing file: %s (%s) ===", file.Filename, file.ID)

	// 1. Get all chunks for this file
	chunks, err := db.GetChunksByFileID(file.ID)
	if err != nil {
		result.Error = fmt.Errorf("failed to get chunks: %w", err)
		return result
	}

	// Sort chunks by index
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].ChunkIndex < chunks[j].ChunkIndex
	})

	log.Printf("Total chunks to reconstruct: %d", len(chunks))

	// 2. Process each chunk
	reconstructedChunks := make([]chunker.ChunkResult, 0, len(chunks))
	totalMissing := 0
	totalRecovered := 0

	for _, chunk := range chunks {
		chunkData, missing, recovered, err := r.reconstructChunk(file, &chunk)
		if err != nil {
			result.Error = fmt.Errorf("failed to reconstruct chunk %d: %w", chunk.ChunkIndex, err)
			return result
		}

		reconstructedChunks = append(reconstructedChunks, chunker.ChunkResult{
			Index: chunk.ChunkIndex,
			Data:  chunkData,
			Size:  int64(len(chunkData)),
		})

		totalMissing += missing
		totalRecovered += recovered
	}

	// 3. Merge all chunks back into the complete file
	fileData := chunker.MergeChunks(reconstructedChunks)

	// 4. Validate SHA-256 against original checksum
	reconstructedChecksum := checksum.CalculateFromBytes(fileData)
	isValid := reconstructedChecksum == file.OriginalChecksum

	log.Printf("=== Reconstruction complete ===")
	log.Printf("  File size: %d bytes", len(fileData))
	log.Printf("  Original SHA-256:       %s", file.OriginalChecksum)
	log.Printf("  Reconstructed SHA-256:  %s", reconstructedChecksum)
	log.Printf("  Valid: %v", isValid)
	log.Printf("  Missing shards: %d", totalMissing)
	log.Printf("  Recovered shards: %d", totalRecovered)

	if !isValid {
		result.Error = fmt.Errorf(
			"integrity check FAILED: expected %s, got %s",
			file.OriginalChecksum, reconstructedChecksum,
		)
		result.Valid = false
		result.FileData = nil // Failsafe: never return corrupted bytes
		return result
	}

	result.FileData = fileData
	result.Valid = true
	result.Reconstructed = totalRecovered > 0
	result.Checksum = reconstructedChecksum
	result.MissingShards = totalMissing
	result.RecoveredShards = totalRecovered
	return result
}

// reconstructChunk handles a single chunk: fetch available shards, reconstruct if needed.
func (r *Reconstructor) reconstructChunk(file *models.File, chunk *models.Chunk) (data []byte, missing int, recovered int, err error) {
	log.Printf("  Chunk %d: fetching shards...", chunk.ChunkIndex)

	// Get shard metadata for this chunk
	shardRecords, err := db.GetShardsByChunkID(chunk.ID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to get shard records: %w", err)
	}

	totalShards := r.encoder.TotalShards() // 4 (3 data + 1 parity)
	shards := make([][]byte, totalShards)
	missingCount := 0

	// Try to fetch each shard from its assigned node
	for _, sr := range shardRecords {
		// Check if the node is online
		node, nodeErr := db.GetNode(sr.NodeID)
		if nodeErr != nil || node.Status != "ONLINE" {
			log.Printf("    Shard %d: node %s is OFFLINE -> MISSING", sr.ShardIndex, sr.NodeID)
			shards[sr.ShardIndex] = nil
			missingCount++
			continue
		}

		// Fetch shard via gRPC
		shardData, fetchErr := r.shardMgr.RetrieveShard(sr.NodeID, file.ID, chunk.ID, sr.ShardIndex)
		if fetchErr != nil {
			log.Printf("    Shard %d: fetch FAILED from %s: %v -> MISSING", sr.ShardIndex, sr.NodeID, fetchErr)
			shards[sr.ShardIndex] = nil
			missingCount++
			continue
		}

		// Verify shard checksum (failsafe rule: corrupt shard is treated as missing)
		if !checksum.Verify(shardData, sr.Checksum) {
			log.Printf("    Shard %d: checksum MISMATCH -> CORRUPTED -> treating as MISSING", sr.ShardIndex)
			shards[sr.ShardIndex] = nil
			missingCount++
			continue
		}

		shards[sr.ShardIndex] = shardData
		log.Printf("    Shard %d: [OK] retrieved from %s (%d bytes)", sr.ShardIndex, sr.NodeID, len(shardData))
	}

	// Check if we have enough shards (need at least K data shards)
	availableCount := totalShards - missingCount
	if availableCount < r.encoder.DataShards() {
		return nil, missingCount, 0, fmt.Errorf(
			"not enough shards: have %d, need %d (missing %d)",
			availableCount, r.encoder.DataShards(), missingCount,
		)
	}

	// Decode (reconstruct missing shards using Reed-Solomon)
	chunkData, decodeErr := r.encoder.Decode(shards, chunk.ChunkSize)
	if decodeErr != nil {
		return nil, missingCount, 0, fmt.Errorf("Reed-Solomon decode failed: %w", decodeErr)
	}

	recoveredCount := 0
	if missingCount > 0 {
		recoveredCount = missingCount
		log.Printf("    [OK] Reconstructed %d missing shard(s) via Reed-Solomon", recoveredCount)
	}

	log.Printf("  Chunk %d: [OK] restored (%d bytes)", chunk.ChunkIndex, len(chunkData))
	return chunkData, missingCount, recoveredCount, nil
}

// PreflightCheck verifies if a download is feasible before attempting reconstruction.
type PreflightResult struct {
	Feasible       bool     `json:"feasible"`
	TotalChunks    int      `json:"total_chunks"`
	TotalShards    int      `json:"total_shards"`
	AvailableNodes int      `json:"available_nodes"`
	MissingNodes   []string `json:"missing_nodes"`
	Status         string   `json:"status"` // OK | DEGRADED | IMPOSSIBLE
	Message        string   `json:"message"`
}

func (r *Reconstructor) PreflightCheck(file *models.File) (*PreflightResult, error) {
	result := &PreflightResult{
		TotalChunks: file.TotalChunks,
	}

	nodes, err := db.ListNodes()
	if err != nil {
		return nil, err
	}

	onlineCount := 0
	for _, n := range nodes {
		if n.Status == "ONLINE" {
			onlineCount++
		} else {
			result.MissingNodes = append(result.MissingNodes, n.ID)
		}
	}

	result.AvailableNodes = onlineCount
	result.TotalShards = file.TotalChunks * (file.DataShards + file.ParityShards)

	if onlineCount == len(nodes) {
		result.Feasible = true
		result.Status = "OK"
		result.Message = "All nodes online — direct download"
	} else if onlineCount >= file.DataShards {
		result.Feasible = true
		result.Status = "DEGRADED"
		result.Message = fmt.Sprintf(
			"%d node(s) offline — will reconstruct using Reed-Solomon",
			len(nodes)-onlineCount,
		)
	} else {
		result.Feasible = false
		result.Status = "IMPOSSIBLE"
		result.Message = fmt.Sprintf(
			"Only %d nodes online, need at least %d — recovery impossible",
			onlineCount, file.DataShards,
		)
	}

	return result, nil
}
