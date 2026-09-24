package api

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/surya/dist-file-storage/coordinator/internal/checksum"
	"github.com/surya/dist-file-storage/coordinator/internal/chunker"
	"github.com/surya/dist-file-storage/coordinator/internal/db"
	"github.com/surya/dist-file-storage/coordinator/internal/erasure"
	"github.com/surya/dist-file-storage/coordinator/internal/models"
	"github.com/surya/dist-file-storage/coordinator/internal/reconstruction"
	"github.com/surya/dist-file-storage/coordinator/internal/shardmanager"
)

type Handler struct {
	shardMgr *shardmanager.ShardManager
	encoder  *erasure.Encoder
}

func NewHandler(sm *shardmanager.ShardManager, enc *erasure.Encoder) *Handler {
	return &Handler{shardMgr: sm, encoder: enc}
}

// ─── Upload ────────────────────────────────────────────

func (h *Handler) UploadFile(c *gin.Context) {
	// 1. Read uploaded file first so the client's request stream is cleanly consumed
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided in form field 'file'"})
		return
	}
	defer file.Close()

	fileData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file data"})
		return
	}

	// 2. Check if all 4 nodes are online before upload (each node receives 1 shard)
	onlineNodes, err := db.GetOnlineNodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check storage node status"})
		return
	}
	if len(onlineNodes) < 4 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": fmt.Sprintf("Cluster degraded: %d/4 nodes online. Upload requires all 4 nodes to be online. Please start all nodes in the dashboard before uploading.", len(onlineNodes)),
		})
		return
	}

	log.Printf("Upload request: %s (%d bytes)", header.Filename, len(fileData))

	// 2. Calculate file-level SHA-256 checksum
	fileChecksum := checksum.CalculateFromBytes(fileData)
	log.Printf("Calculated file SHA-256: %s", fileChecksum)

	// 3. Chunk the file into ~4MB pieces
	chunks, err := chunker.SplitData(fileData, chunker.DefaultChunkSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to chunk file"})
		return
	}

	log.Printf("Divided into %d chunk(s) (chunk size: %d bytes)", len(chunks), chunker.DefaultChunkSize)

	// 4. Create file record in PostgreSQL
	fileID := uuid.New().String()
	now := time.Now()
	fileRecord := &models.File{
		ID:               fileID,
		Filename:         header.Filename,
		FileSize:         int64(len(fileData)),
		OriginalChecksum: fileChecksum,
		TotalChunks:      len(chunks),
		DataShards:       h.encoder.DataShards(),
		ParityShards:     h.encoder.ParityShards(),
		ChunkSize:        chunker.DefaultChunkSize,
		Status:           "UPLOADING",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := db.InsertFile(fileRecord); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file metadata"})
		return
	}

	// 5. Process each chunk: Reed-Solomon encode -> distribute shards via gRPC
	totalShards := 0
	for _, chk := range chunks {
		chunkID := uuid.New().String()

		// Save chunk record
		chunkRecord := &models.Chunk{
			ID:         chunkID,
			FileID:     fileID,
			ChunkIndex: chk.Index,
			ChunkSize:  chk.Size,
			CreatedAt:  now,
		}
		if err := db.InsertChunk(chunkRecord); err != nil {
			_ = db.UpdateFileStatus(fileID, "FAILED")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save chunk metadata"})
			return
		}

		// Reed-Solomon encode: produces 3 data shards + 1 parity shard
		shards, err := h.encoder.Encode(chk.Data)
		if err != nil {
			_ = db.UpdateFileStatus(fileID, "FAILED")
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Reed-Solomon encoding failed: %v", err)})
			return
		}

		// Distribute each shard to its designated node
		for shardIdx, shardData := range shards {
			nodeID := shardmanager.GetNodeAssignment(shardIdx)
			shardType := shardmanager.GetShardType(shardIdx, h.encoder.DataShards())
			shardChecksum := checksum.CalculateFromBytes(shardData)

			// Store shard on node via gRPC
			err := h.shardMgr.StoreShard(nodeID, fileID, chunkID, shardIdx, shardType, shardData)
			if err != nil {
				log.Printf("[WARN] Failed to store shard %d on %s: %v", shardIdx, nodeID, err)
				_ = db.UpdateFileStatus(fileID, "FAILED")
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("Failed to store shard %d on %s: %v", shardIdx, nodeID, err),
				})
				return
			}

			// Save shard metadata record in PostgreSQL
			shardRecord := &models.Shard{
				ID:         uuid.New().String(),
				ChunkID:    chunkID,
				FileID:     fileID,
				ShardIndex: shardIdx,
				ShardType:  shardType,
				NodeID:     nodeID,
				ShardSize:  int64(len(shardData)),
				Checksum:   shardChecksum,
				Status:     "STORED",
				CreatedAt:  now,
			}
			if err := db.InsertShard(shardRecord); err != nil {
				log.Printf("[WARN] Failed to save shard metadata: %v", err)
			}

			totalShards++
		}

		log.Printf("[OK] Chunk %d: encoded and distributed (%d shards)", chk.Index, len(shards))
	}

	// 6. Mark file as HEALTHY in PostgreSQL
	_ = db.UpdateFileStatus(fileID, "HEALTHY")

	c.JSON(http.StatusOK, models.UploadResult{
		FileID:   fileID,
		Filename: header.Filename,
		FileSize: int64(len(fileData)),
		Checksum: fileChecksum,
		Chunks:   len(chunks),
		Shards:   totalShards,
		Status:   "HEALTHY",
	})
}

// ─── List Files ────────────────────────────────────────

func (h *Handler) ListFiles(c *gin.Context) {
	files, err := db.ListFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if files == nil {
		files = []models.File{}
	}
	c.JSON(http.StatusOK, files)
}

// ─── Get File ──────────────────────────────────────────

func (h *Handler) GetFile(c *gin.Context) {
	fileID := c.Param("id")
	file, err := db.GetFile(fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	shards, _ := db.GetShardsByFileID(fileID)
	if shards == nil {
		shards = []models.Shard{}
	}

	c.JSON(http.StatusOK, gin.H{
		"file":   file,
		"shards": shards,
	})
}

// ─── Download & Reconstruction ─────────────────────────

func (h *Handler) DownloadFile(c *gin.Context) {
	fileID := c.Param("id")

	// 1. Get file metadata
	file, err := db.GetFile(fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Download request: %s (%s)", file.Filename, file.ID)

	// 2. Preflight check — check if enough nodes are online
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

	// 3. Reconstruct file: fetches shards, checks checksums, reconstructs missing shards via Reed-Solomon
	result := reconstructor.ReconstructFile(file)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "File reconstruction failed",
			"reason": result.Error.Error(),
			"valid":  result.Valid,
		})
		return
	}

	// 4. Validate failsafe integrity check (SHA-256 against original checksum in DB)
	if !result.Valid {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "File integrity check failed — reconstructed data did not match original checksum",
			"valid": false,
		})
		return
	}

	// 5. Send reconstructed/verified file to user
	log.Printf("[OK] Serving file: %s (%d bytes, reconstructed=%v)",
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

// ─── Delete File ───────────────────────────────────────

func (h *Handler) DeleteFile(c *gin.Context) {
	fileID := c.Param("id")

	// Verify file exists
	_, err := db.GetFile(fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Delete shards from all storage nodes
	nodeIDs := []string{"node-1", "node-2", "node-3", "node-4"}
	for _, nodeID := range nodeIDs {
		node, err := db.GetNode(nodeID)
		if err != nil || node.Status != "ONLINE" {
			log.Printf("[WARN] Skipping delete on %s (offline or not found)", nodeID)
			continue
		}
		if err := h.shardMgr.DeleteFileFromNode(nodeID, fileID); err != nil {
			log.Printf("[WARN] Failed to delete shards from %s: %v", nodeID, err)
		}
	}

	// Delete from PostgreSQL (cascading deletes will remove chunks & shards)
	if err := db.DeleteFile(fileID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
}

// ─── Nodes & Simulation ────────────────────────────────

func (h *Handler) ListNodes(c *gin.Context) {
	nodes, err := db.ListNodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if nodes == nil {
		nodes = []models.Node{}
	}
	c.JSON(http.StatusOK, nodes)
}

func (h *Handler) GetNode(c *gin.Context) {
	nodeID := c.Param("id")
	node, err := db.GetNode(nodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, node)
}

func (h *Handler) StopNode(c *gin.Context) {
	nodeID := c.Param("id")
	if err := db.UpdateNodeStatus(nodeID, "OFFLINE"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("Node %s set to OFFLINE", nodeID)
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Node %s is now OFFLINE", nodeID), "status": "OFFLINE"})
}

func (h *Handler) StartNode(c *gin.Context) {
	nodeID := c.Param("id")
	if err := db.UpdateNodeStatus(nodeID, "ONLINE"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("Node %s set to ONLINE", nodeID)
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Node %s is now ONLINE", nodeID), "status": "ONLINE"})
}

// ─── Health & Cluster Stats ────────────────────────────

func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "coordinator"})
}

func (h *Handler) GetStats(c *gin.Context) {
	files, _ := db.ListFiles()
	nodes, _ := db.ListNodes()

	onlineNodes := 0
	totalStorage := int64(0)
	usedStorage := int64(0)
	for _, n := range nodes {
		if n.Status == "ONLINE" {
			onlineNodes++
		}
		totalStorage += n.TotalStorage
		usedStorage += n.UsedStorage
	}

	totalFiles := len(files)
	totalSize := int64(0)
	for _, f := range files {
		totalSize += f.FileSize
	}

	c.JSON(http.StatusOK, gin.H{
		"total_files":   totalFiles,
		"total_size":    totalSize,
		"total_nodes":   len(nodes),
		"online_nodes":  onlineNodes,
		"total_storage": totalStorage,
		"used_storage":  usedStorage,
	})
}
