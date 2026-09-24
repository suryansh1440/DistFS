# Step 08 — Download Flow, Reconstruction & Validation

## Goal
Implement the complete download flow: fetch shards from nodes, detect missing shards, reconstruct using Reed-Solomon, validate checksums, and serve the file — or return an error if recovery fails.

---

## This Is the "Failsafe" Step

This step implements the most critical path:

```
Download Request
      │
      ▼
 Check node status
      │
      ▼
┌─────┴──────────┐
│                │
All available    Node(s) offline
│                │
▼                ▼
Fetch shards     Enough shards
from nodes       available?
│                │
│           YES ─┤── NO ──► ERROR 503
│                │
│                ▼
│           Reed-Solomon
│           reconstruction
│                │
└───────┬────────┘
        ▼
  Rebuild full file
        │
        ▼
  SHA-256 verify
        │
   ┌────┴────┐
   │         │
  PASS      FAIL
   │         │
   ▼         ▼
Send file   ERROR 500
to user     "integrity
            check failed"
```

---

## Files to Create

### `coordinator/internal/reconstruction/reconstruct.go`

```go
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

// ReconstructionResult holds the result of a file reconstruction attempt.
type ReconstructionResult struct {
    FileData       []byte
    Valid          bool
    Reconstructed  bool   // true if any shard was recovered via Reed-Solomon
    Checksum       string // SHA-256 of reconstructed file
    MissingShards  int
    RecoveredShards int
    Error          error
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

    log.Printf("═══ Reconstructing file: %s (%s) ═══", file.Filename, file.ID)

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

    log.Printf("═══ Reconstruction complete ═══")
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
        result.FileData = nil // Do NOT return corrupted data
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
    shardChecksums := make(map[int]string) // shardIndex → expected checksum
    missingCount := 0

    // Try to fetch each shard from its assigned node
    for _, sr := range shardRecords {
        shardChecksums[sr.ShardIndex] = sr.Checksum

        // Check if the node is online
        node, nodeErr := db.GetNode(sr.NodeID)
        if nodeErr != nil || node.Status != "ONLINE" {
            log.Printf("    Shard %d: node %s is OFFLINE → MISSING", sr.ShardIndex, sr.NodeID)
            shards[sr.ShardIndex] = nil
            missingCount++
            continue
        }

        // Fetch shard via gRPC
        shardData, fetchErr := r.shardMgr.RetrieveShard(sr.NodeID, file.ID, chunk.ID, sr.ShardIndex)
        if fetchErr != nil {
            log.Printf("    Shard %d: fetch FAILED from %s: %v → MISSING", sr.ShardIndex, sr.NodeID, fetchErr)
            shards[sr.ShardIndex] = nil
            missingCount++
            continue
        }

        // Verify shard checksum
        if !checksum.Verify(shardData, sr.Checksum) {
            log.Printf("    Shard %d: checksum MISMATCH → CORRUPTED → treating as MISSING", sr.ShardIndex)
            shards[sr.ShardIndex] = nil
            missingCount++
            continue
        }

        shards[sr.ShardIndex] = shardData
        log.Printf("    Shard %d: ✓ retrieved from %s (%d bytes)", sr.ShardIndex, sr.NodeID, len(shardData))
    }

    // Check if we have enough shards
    availableCount := totalShards - missingCount
    if availableCount < r.encoder.DataShards() {
        return nil, missingCount, 0, fmt.Errorf(
            "not enough shards: have %d, need %d (missing %d)",
            availableCount, r.encoder.DataShards(), missingCount,
        )
    }

    // Decode (reconstruct missing shards if needed)
    chunkData, decodeErr := r.encoder.Decode(shards, chunk.ChunkSize)
    if decodeErr != nil {
        return nil, missingCount, 0, fmt.Errorf("Reed-Solomon decode failed: %w", decodeErr)
    }

    recoveredCount := 0
    if missingCount > 0 {
        recoveredCount = missingCount
        log.Printf("    ✓ Reconstructed %d missing shard(s) via Reed-Solomon", recoveredCount)
    }

    log.Printf("  Chunk %d: ✓ restored (%d bytes)", chunk.ChunkIndex, len(chunkData))
    return chunkData, missingCount, recoveredCount, nil
}

// PreflightCheck verifies if a download is feasible before attempting reconstruction.
type PreflightResult struct {
    Feasible       bool              `json:"feasible"`
    TotalChunks    int               `json:"total_chunks"`
    TotalShards    int               `json:"total_shards"`
    AvailableNodes int               `json:"available_nodes"`
    MissingNodes   []string          `json:"missing_nodes"`
    Status         string            `json:"status"` // OK | DEGRADED | IMPOSSIBLE
    Message        string            `json:"message"`
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
```

---

### Update `coordinator/internal/api/handlers.go` — Replace DownloadFile

Replace the placeholder `DownloadFile` handler with:

```go
func (h *Handler) DownloadFile(c *gin.Context) {
    fileID := c.Param("id")

    // 1. Get file metadata
    file, err := db.GetFile(fileID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
        return
    }

    log.Printf("Download request: %s (%s)", file.Filename, file.ID)

    // 2. Preflight check — is reconstruction feasible?
    reconstructor := reconstruction.NewReconstructor(h.shardMgr, h.encoder)
    preflight, err := reconstructor.PreflightCheck(file)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Preflight check failed"})
        return
    }

    if !preflight.Feasible {
        c.JSON(http.StatusServiceUnavailable, gin.H{
            "error":     "Download not possible",
            "reason":    preflight.Message,
            "status":    preflight.Status,
            "missing":   preflight.MissingNodes,
            "available": preflight.AvailableNodes,
            "required":  file.DataShards,
        })
        return
    }

    // 3. Reconstruct the file
    result := reconstructor.ReconstructFile(file)
    if result.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":  "File reconstruction failed",
            "reason": result.Error.Error(),
            "valid":  result.Valid,
        })
        return
    }

    // 4. Final validation (should always pass if reconstruction succeeded)
    if !result.Valid {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "File integrity check failed — file may be corrupted",
            "valid": false,
        })
        return
    }

    // 5. Send the file to the user
    log.Printf("✓ Sending file: %s (%d bytes, reconstructed=%v)",
        file.Filename, len(result.FileData), result.Reconstructed)

    c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
    c.Header("Content-Type", "application/octet-stream")
    c.Header("X-Original-Checksum", file.OriginalChecksum)
    c.Header("X-Reconstructed-Checksum", result.Checksum)
    c.Header("X-Reconstructed", fmt.Sprintf("%v", result.Reconstructed))
    c.Header("X-Missing-Shards", fmt.Sprintf("%d", result.MissingShards))
    c.Header("X-Recovered-Shards", fmt.Sprintf("%d", result.RecoveredShards))

    c.Data(http.StatusOK, "application/octet-stream", result.FileData)
}
```

**Remember to add the import:**
```go
import "github.com/surya/dist-file-storage/coordinator/internal/reconstruction"
```

---

## Download Flow — All Nodes Healthy

```
GET /api/files/:id/download
         │
         ▼
   File metadata from PostgreSQL
         │
         ▼
   Preflight: 4/4 nodes online → OK
         │
         ▼
   For each chunk (25 chunks for 100MB):
       │
       ├── Fetch D1 from Node 1 ✓
       ├── Fetch D2 from Node 2 ✓
       ├── Fetch D3 from Node 3 ✓
       ├── (Skip P1 — not needed when all data shards available)
       │
       └── Verify each shard checksum ✓
         │
         ▼
   Merge 25 chunks → 100MB file
         │
         ▼
   SHA-256(file) == original checksum? ✓
         │
         ▼
   HTTP 200 + file data
```

## Download Flow — Node 2 Offline

```
GET /api/files/:id/download
         │
         ▼
   File metadata from PostgreSQL
         │
         ▼
   Preflight: 3/4 nodes online → DEGRADED (feasible)
         │
         ▼
   For each chunk:
       │
       ├── Fetch D1 from Node 1 ✓
       ├── Fetch D2 from Node 2 ✗ (OFFLINE)
       ├── Fetch D3 from Node 3 ✓
       ├── Fetch P1 from Node 4 ✓
       │
       └── Reed-Solomon: D1 + D3 + P1 → reconstruct D2 ✓
         │
         ▼
   Merge chunks → file
         │
         ▼
   SHA-256 verify ✓
         │
         ▼
   HTTP 200 + file data
   (headers: X-Reconstructed: true, X-Missing-Shards: 25)
```

## Download Flow — 2 Nodes Offline (FAILURE)

```
GET /api/files/:id/download
         │
         ▼
   Preflight: 2/4 nodes online → IMPOSSIBLE
         │
         ▼
   HTTP 503 Service Unavailable
   {
     "error": "Download not possible",
     "reason": "Only 2 nodes online, need at least 3",
     "missing": ["node-2", "node-3"]
   }
```

---

## Validation Layers

| Level | What | When | Action on Failure |
|:---:|------|------|-------------------|
| 1 | Shard checksum (SHA-256) | After fetching each shard from a node | Treat shard as MISSING → attempt RS reconstruction |
| 2 | Reed-Solomon verify | After reconstruction | Return decode error |
| 3 | File checksum (SHA-256) | After merging all chunks | Do NOT send file → return integrity error |

---

## Verification

```bash
cd coordinator
go build ./...

# End-to-end test sequence (after Docker Compose — Step 09):

# 1. Upload a file
curl -X POST http://localhost:8080/api/files/upload -F "file=@test.txt"
# → Returns file_id

# 2. Download normally (all nodes online)
curl http://localhost:8080/api/files/<file_id>/download -o downloaded.txt
# → Should match original

# 3. Stop a node
curl -X POST http://localhost:8080/api/nodes/node-2/stop

# 4. Download with reconstruction
curl -v http://localhost:8080/api/files/<file_id>/download -o recovered.txt
# → Should still work
# → X-Reconstructed: true header present

# 5. Stop another node
curl -X POST http://localhost:8080/api/nodes/node-3/stop

# 6. Attempt download (should fail)
curl -v http://localhost:8080/api/files/<file_id>/download
# → HTTP 503 with error message

# 7. Compare checksums
sha256sum test.txt downloaded.txt recovered.txt
# All should match
```

---

## What This Step Achieves
- Complete download/reconstruction pipeline
- Preflight check before attempting reconstruction
- Shard-level checksum verification
- Reed-Solomon reconstruction for missing shards
- File-level checksum verification (failsafe)
- Clear error responses for impossible reconstructions
- Custom HTTP headers indicating reconstruction details
