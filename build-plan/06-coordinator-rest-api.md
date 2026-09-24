# Step 06 — Coordinator REST API & Shard Manager

## Goal
Build the REST API endpoints for file upload/download/delete/list, the shard manager that distributes shards to storage nodes via gRPC, and the database query layer.

---

## REST API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/files/upload` | Upload a file (multipart/form-data) |
| `GET` | `/api/files` | List all uploaded files |
| `GET` | `/api/files/:id` | Get file metadata |
| `GET` | `/api/files/:id/download` | Download / reconstruct a file |
| `DELETE` | `/api/files/:id` | Delete a file and all its shards |
| `GET` | `/api/nodes` | List all storage nodes |
| `GET` | `/api/nodes/:id` | Get node details |
| `POST` | `/api/nodes/:id/stop` | Simulate node failure (set OFFLINE) |
| `POST` | `/api/nodes/:id/start` | Bring node back online |
| `GET` | `/api/health` | Coordinator health check |
| `GET` | `/api/stats` | Cluster-wide statistics |

---

## Files to Create

### `coordinator/internal/db/queries.go`

```go
package db

import (
    "database/sql"
    "fmt"
    "time"

    "github.com/surya/dist-file-storage/coordinator/internal/models"
)

// ─── Files ─────────────────────────────────────────────

func InsertFile(f *models.File) error {
    _, err := DB.Exec(`
        INSERT INTO files (id, filename, file_size, original_checksum, total_chunks,
                           data_shards, parity_shards, chunk_size, status, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
        f.ID, f.Filename, f.FileSize, f.OriginalChecksum, f.TotalChunks,
        f.DataShards, f.ParityShards, f.ChunkSize, f.Status, f.CreatedAt, f.UpdatedAt,
    )
    return err
}

func UpdateFileStatus(fileID, status string) error {
    _, err := DB.Exec(`UPDATE files SET status = $1, updated_at = $2 WHERE id = $3`,
        status, time.Now(), fileID)
    return err
}

func GetFile(fileID string) (*models.File, error) {
    f := &models.File{}
    err := DB.QueryRow(`
        SELECT id, filename, file_size, original_checksum, total_chunks,
               data_shards, parity_shards, chunk_size, status, created_at, updated_at
        FROM files WHERE id = $1`, fileID).Scan(
        &f.ID, &f.Filename, &f.FileSize, &f.OriginalChecksum, &f.TotalChunks,
        &f.DataShards, &f.ParityShards, &f.ChunkSize, &f.Status, &f.CreatedAt, &f.UpdatedAt,
    )
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("file not found: %s", fileID)
    }
    return f, err
}

func ListFiles() ([]models.File, error) {
    rows, err := DB.Query(`
        SELECT id, filename, file_size, original_checksum, total_chunks,
               data_shards, parity_shards, chunk_size, status, created_at, updated_at
        FROM files ORDER BY created_at DESC`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var files []models.File
    for rows.Next() {
        var f models.File
        err := rows.Scan(&f.ID, &f.Filename, &f.FileSize, &f.OriginalChecksum, &f.TotalChunks,
            &f.DataShards, &f.ParityShards, &f.ChunkSize, &f.Status, &f.CreatedAt, &f.UpdatedAt)
        if err != nil {
            return nil, err
        }
        files = append(files, f)
    }
    return files, nil
}

func DeleteFile(fileID string) error {
    _, err := DB.Exec(`DELETE FROM files WHERE id = $1`, fileID)
    return err
}

// ─── Chunks ────────────────────────────────────────────

func InsertChunk(c *models.Chunk) error {
    _, err := DB.Exec(`
        INSERT INTO chunks (id, file_id, chunk_index, chunk_size, created_at)
        VALUES ($1, $2, $3, $4, $5)`,
        c.ID, c.FileID, c.ChunkIndex, c.ChunkSize, c.CreatedAt,
    )
    return err
}

func GetChunksByFileID(fileID string) ([]models.Chunk, error) {
    rows, err := DB.Query(`
        SELECT id, file_id, chunk_index, chunk_size, created_at
        FROM chunks WHERE file_id = $1 ORDER BY chunk_index`, fileID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var chunks []models.Chunk
    for rows.Next() {
        var c models.Chunk
        err := rows.Scan(&c.ID, &c.FileID, &c.ChunkIndex, &c.ChunkSize, &c.CreatedAt)
        if err != nil {
            return nil, err
        }
        chunks = append(chunks, c)
    }
    return chunks, nil
}

// ─── Shards ────────────────────────────────────────────

func InsertShard(s *models.Shard) error {
    _, err := DB.Exec(`
        INSERT INTO shards (id, chunk_id, file_id, shard_index, shard_type,
                            node_id, shard_size, checksum, status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
        s.ID, s.ChunkID, s.FileID, s.ShardIndex, s.ShardType,
        s.NodeID, s.ShardSize, s.Checksum, s.Status, s.CreatedAt,
    )
    return err
}

func GetShardsByChunkID(chunkID string) ([]models.Shard, error) {
    rows, err := DB.Query(`
        SELECT id, chunk_id, file_id, shard_index, shard_type,
               node_id, shard_size, checksum, status, created_at
        FROM shards WHERE chunk_id = $1 ORDER BY shard_index`, chunkID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var shards []models.Shard
    for rows.Next() {
        var s models.Shard
        err := rows.Scan(&s.ID, &s.ChunkID, &s.FileID, &s.ShardIndex, &s.ShardType,
            &s.NodeID, &s.ShardSize, &s.Checksum, &s.Status, &s.CreatedAt)
        if err != nil {
            return nil, err
        }
        shards = append(shards, s)
    }
    return shards, nil
}

func GetShardsByFileID(fileID string) ([]models.Shard, error) {
    rows, err := DB.Query(`
        SELECT id, chunk_id, file_id, shard_index, shard_type,
               node_id, shard_size, checksum, status, created_at
        FROM shards WHERE file_id = $1 ORDER BY shard_index`, fileID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var shards []models.Shard
    for rows.Next() {
        var s models.Shard
        err := rows.Scan(&s.ID, &s.ChunkID, &s.FileID, &s.ShardIndex, &s.ShardType,
            &s.NodeID, &s.ShardSize, &s.Checksum, &s.Status, &s.CreatedAt)
        if err != nil {
            return nil, err
        }
        shards = append(shards, s)
    }
    return shards, nil
}

// ─── Nodes ─────────────────────────────────────────────

func GetNode(nodeID string) (*models.Node, error) {
    n := &models.Node{}
    err := DB.QueryRow(`
        SELECT id, name, grpc_address, status, total_storage, used_storage,
               shard_count, last_heartbeat, created_at, updated_at
        FROM nodes WHERE id = $1`, nodeID).Scan(
        &n.ID, &n.Name, &n.GRPCAddress, &n.Status, &n.TotalStorage, &n.UsedStorage,
        &n.ShardCount, &n.LastHeartbeat, &n.CreatedAt, &n.UpdatedAt,
    )
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("node not found: %s", nodeID)
    }
    return n, err
}

func ListNodes() ([]models.Node, error) {
    rows, err := DB.Query(`
        SELECT id, name, grpc_address, status, total_storage, used_storage,
               shard_count, last_heartbeat, created_at, updated_at
        FROM nodes ORDER BY name`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var nodes []models.Node
    for rows.Next() {
        var n models.Node
        err := rows.Scan(&n.ID, &n.Name, &n.GRPCAddress, &n.Status, &n.TotalStorage, &n.UsedStorage,
            &n.ShardCount, &n.LastHeartbeat, &n.CreatedAt, &n.UpdatedAt)
        if err != nil {
            return nil, err
        }
        nodes = append(nodes, n)
    }
    return nodes, nil
}

func UpdateNodeStatus(nodeID, status string) error {
    _, err := DB.Exec(`UPDATE nodes SET status = $1, updated_at = $2 WHERE id = $3`,
        status, time.Now(), nodeID)
    return err
}

func UpdateNodeStats(nodeID string, usedStorage int64, shardCount int) error {
    _, err := DB.Exec(`
        UPDATE nodes SET used_storage = $1, shard_count = $2,
               last_heartbeat = $3, updated_at = $4 WHERE id = $5`,
        usedStorage, shardCount, time.Now(), time.Now(), nodeID)
    return err
}

func GetOnlineNodes() ([]models.Node, error) {
    rows, err := DB.Query(`
        SELECT id, name, grpc_address, status, total_storage, used_storage,
               shard_count, last_heartbeat, created_at, updated_at
        FROM nodes WHERE status = 'ONLINE' ORDER BY name`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var nodes []models.Node
    for rows.Next() {
        var n models.Node
        err := rows.Scan(&n.ID, &n.Name, &n.GRPCAddress, &n.Status, &n.TotalStorage, &n.UsedStorage,
            &n.ShardCount, &n.LastHeartbeat, &n.CreatedAt, &n.UpdatedAt)
        if err != nil {
            return nil, err
        }
        nodes = append(nodes, n)
    }
    return nodes, nil
}
```

---

### `coordinator/internal/shardmanager/manager.go`

```go
package shardmanager

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "log"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"

    pb "github.com/surya/dist-file-storage/proto/gen/storagepb"
    "github.com/surya/dist-file-storage/coordinator/internal/checksum"
    "github.com/surya/dist-file-storage/coordinator/internal/db"
    "github.com/surya/dist-file-storage/coordinator/internal/models"
)

const grpcChunkSize = 64 * 1024 // 64KB per stream message

// ShardManager handles distributing and retrieving shards from storage nodes.
type ShardManager struct {
    connections map[string]pb.StorageNodeClient // nodeID → gRPC client
}

func NewShardManager() *ShardManager {
    return &ShardManager{
        connections: make(map[string]pb.StorageNodeClient),
    }
}

// ConnectToNodes establishes gRPC connections to all nodes.
func (sm *ShardManager) ConnectToNodes() error {
    nodes, err := db.ListNodes()
    if err != nil {
        return fmt.Errorf("failed to list nodes: %w", err)
    }

    for _, node := range nodes {
        conn, err := grpc.Dial(
            node.GRPCAddress,
            grpc.WithTransportCredentials(insecure.NewCredentials()),
            grpc.WithDefaultCallOptions(
                grpc.MaxCallRecvMsgSize(8*1024*1024),
                grpc.MaxCallSendMsgSize(8*1024*1024),
            ),
        )
        if err != nil {
            log.Printf("⚠ Failed to connect to node %s (%s): %v", node.ID, node.GRPCAddress, err)
            continue
        }

        sm.connections[node.ID] = pb.NewStorageNodeClient(conn)
        log.Printf("✓ Connected to node %s at %s", node.ID, node.GRPCAddress)
    }

    return nil
}

// StoreShard sends a shard to the designated storage node via gRPC client-streaming.
func (sm *ShardManager) StoreShard(nodeID, fileID, chunkID string, shardIndex int, shardType string, data []byte) error {
    client, ok := sm.connections[nodeID]
    if !ok {
        return fmt.Errorf("no connection to node %s", nodeID)
    }

    // Check if node is online
    node, err := db.GetNode(nodeID)
    if err != nil {
        return err
    }
    if node.Status != "ONLINE" {
        return fmt.Errorf("node %s is OFFLINE", nodeID)
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    stream, err := client.StoreShard(ctx)
    if err != nil {
        return fmt.Errorf("failed to open store stream to %s: %w", nodeID, err)
    }

    // Send metadata first
    shardChecksum := checksum.CalculateFromBytes(data)
    err = stream.Send(&pb.StoreShardRequest{
        Payload: &pb.StoreShardRequest_Metadata{
            Metadata: &pb.ShardMetadata{
                FileId:     fileID,
                ChunkId:    chunkID,
                ShardIndex: int32(shardIndex),
                ShardType:  shardType,
                Checksum:   shardChecksum,
                Size:       int64(len(data)),
            },
        },
    })
    if err != nil {
        return fmt.Errorf("failed to send metadata: %w", err)
    }

    // Stream data in 64KB chunks
    for offset := 0; offset < len(data); offset += grpcChunkSize {
        end := offset + grpcChunkSize
        if end > len(data) {
            end = len(data)
        }

        err = stream.Send(&pb.StoreShardRequest{
            Payload: &pb.StoreShardRequest_Data{
                Data: data[offset:end],
            },
        })
        if err != nil {
            return fmt.Errorf("failed to send data chunk: %w", err)
        }
    }

    resp, err := stream.CloseAndRecv()
    if err != nil {
        return fmt.Errorf("failed to close stream: %w", err)
    }

    if !resp.Success {
        return fmt.Errorf("node rejected shard: %s", resp.Message)
    }

    log.Printf("✓ Stored shard %d on %s (checksum: %s...)", shardIndex, nodeID, shardChecksum[:8])
    return nil
}

// RetrieveShard retrieves a shard from a storage node via gRPC server-streaming.
func (sm *ShardManager) RetrieveShard(nodeID, fileID, chunkID string, shardIndex int) ([]byte, error) {
    client, ok := sm.connections[nodeID]
    if !ok {
        return nil, fmt.Errorf("no connection to node %s", nodeID)
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    stream, err := client.RetrieveShard(ctx, &pb.RetrieveShardRequest{
        FileId:     fileID,
        ChunkId:    chunkID,
        ShardIndex: int32(shardIndex),
    })
    if err != nil {
        return nil, fmt.Errorf("failed to open retrieve stream from %s: %w", nodeID, err)
    }

    var buf bytes.Buffer
    for {
        resp, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, fmt.Errorf("failed to receive data from %s: %w", nodeID, err)
        }
        buf.Write(resp.Data)
    }

    return buf.Bytes(), nil
}

// DeleteFileFromNode deletes all shards for a file from a specific node.
func (sm *ShardManager) DeleteFileFromNode(nodeID, fileID string) error {
    client, ok := sm.connections[nodeID]
    if !ok {
        return fmt.Errorf("no connection to node %s", nodeID)
    }

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    resp, err := client.DeleteFileShards(ctx, &pb.DeleteFileShardsRequest{FileId: fileID})
    if err != nil {
        return fmt.Errorf("failed to delete shards from %s: %w", nodeID, err)
    }

    log.Printf("✓ Deleted %d shards from %s for file %s", resp.ShardsDeleted, nodeID, fileID)
    return nil
}

// HealthCheckNode pings a specific node and returns health info.
func (sm *ShardManager) HealthCheckNode(nodeID string) (*pb.HealthCheckResponse, error) {
    client, ok := sm.connections[nodeID]
    if !ok {
        return nil, fmt.Errorf("no connection to node %s", nodeID)
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    resp, err := client.HealthCheck(ctx, &pb.HealthCheckRequest{})
    if err != nil {
        return nil, fmt.Errorf("health check failed for %s: %w", nodeID, err)
    }

    return resp, nil
}

// GetNodeAssignment returns which node a shard should go to.
// Simple round-robin: shard_index → node_id
// Shard 0 → node-1, Shard 1 → node-2, Shard 2 → node-3, Shard 3 → node-4
func GetNodeAssignment(shardIndex int) string {
    nodeIDs := []string{"node-1", "node-2", "node-3", "node-4"}
    return nodeIDs[shardIndex%len(nodeIDs)]
}

// GetShardType returns "DATA" or "PARITY" based on the shard index and config.
func GetShardType(shardIndex, dataShards int) string {
    if shardIndex < dataShards {
        return "DATA"
    }
    return "PARITY"
}
```

---

### `coordinator/internal/api/router.go`

```go
package api

import (
    "github.com/gin-contrib/cors"
    "github.com/gin-gonic/gin"

    "github.com/surya/dist-file-storage/coordinator/internal/erasure"
    "github.com/surya/dist-file-storage/coordinator/internal/shardmanager"
)

func SetupRouter(sm *shardmanager.ShardManager, enc *erasure.Encoder) *gin.Engine {
    r := gin.Default()

    // CORS — allow React dashboard
    r.Use(cors.New(cors.Config{
        AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000"},
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
        AllowCredentials: true,
    }))

    handler := NewHandler(sm, enc)

    api := r.Group("/api")
    {
        // Health
        api.GET("/health", handler.HealthCheck)
        api.GET("/stats", handler.GetStats)

        // Files
        api.POST("/files/upload", handler.UploadFile)
        api.GET("/files", handler.ListFiles)
        api.GET("/files/:id", handler.GetFile)
        api.GET("/files/:id/download", handler.DownloadFile)
        api.DELETE("/files/:id", handler.DeleteFile)

        // Nodes
        api.GET("/nodes", handler.ListNodes)
        api.GET("/nodes/:id", handler.GetNode)
        api.POST("/nodes/:id/stop", handler.StopNode)
        api.POST("/nodes/:id/start", handler.StartNode)
    }

    return r
}
```

---

### `coordinator/internal/api/handlers.go`

```go
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
    // 1. Read uploaded file
    file, header, err := c.Request.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
        return
    }
    defer file.Close()

    fileData, err := io.ReadAll(file)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
        return
    }

    log.Printf("Upload: %s (%d bytes)", header.Filename, len(fileData))

    // 2. Calculate file checksum
    fileChecksum := checksum.CalculateFromBytes(fileData)
    log.Printf("SHA-256: %s", fileChecksum)

    // 3. Chunk the file
    chunks, err := chunker.SplitData(fileData, chunker.DefaultChunkSize)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to chunk file"})
        return
    }

    log.Printf("Chunks: %d (chunk size: %d)", len(chunks), chunker.DefaultChunkSize)

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

    // 5. Process each chunk: encode → distribute shards
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
            db.UpdateFileStatus(fileID, "FAILED")
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save chunk metadata"})
            return
        }

        // Reed-Solomon encode
        shards, err := h.encoder.Encode(chk.Data)
        if err != nil {
            db.UpdateFileStatus(fileID, "FAILED")
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
                log.Printf("⚠ Failed to store shard %d on %s: %v", shardIdx, nodeID, err)
                db.UpdateFileStatus(fileID, "FAILED")
                c.JSON(http.StatusInternalServerError, gin.H{
                    "error": fmt.Sprintf("Failed to store shard %d on %s: %v", shardIdx, nodeID, err),
                })
                return
            }

            // Save shard record in PostgreSQL
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
                log.Printf("⚠ Failed to save shard metadata: %v", err)
            }

            totalShards++
        }

        log.Printf("✓ Chunk %d: encoded and distributed (%d shards)", chk.Index, len(shards))
    }

    // 6. Mark file as HEALTHY
    db.UpdateFileStatus(fileID, "HEALTHY")

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

    // Get shards info
    shards, _ := db.GetShardsByFileID(fileID)

    c.JSON(http.StatusOK, gin.H{
        "file":   file,
        "shards": shards,
    })
}

// ─── Download (covered in detail in Step 08) ───────────

func (h *Handler) DownloadFile(c *gin.Context) {
    // Placeholder — full implementation in Step 08
    c.JSON(http.StatusNotImplemented, gin.H{"error": "Download not yet implemented"})
}

// ─── Delete File ───────────────────────────────────────

func (h *Handler) DeleteFile(c *gin.Context) {
    fileID := c.Param("id")

    // Get file to verify it exists
    _, err := db.GetFile(fileID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
        return
    }

    // Delete shards from all nodes
    nodeIDs := []string{"node-1", "node-2", "node-3", "node-4"}
    for _, nodeID := range nodeIDs {
        node, err := db.GetNode(nodeID)
        if err != nil || node.Status != "ONLINE" {
            log.Printf("⚠ Skipping delete on %s (offline or not found)", nodeID)
            continue
        }
        if err := h.shardMgr.DeleteFileFromNode(nodeID, fileID); err != nil {
            log.Printf("⚠ Failed to delete from %s: %v", nodeID, err)
        }
    }

    // Delete from PostgreSQL (cascading delete handles chunks & shards)
    if err := db.DeleteFile(fileID); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
}

// ─── Nodes ─────────────────────────────────────────────

func (h *Handler) ListNodes(c *gin.Context) {
    nodes, err := db.ListNodes()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
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
    log.Printf("🔴 Node %s set to OFFLINE", nodeID)
    c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Node %s is now OFFLINE", nodeID), "status": "OFFLINE"})
}

func (h *Handler) StartNode(c *gin.Context) {
    nodeID := c.Param("id")
    if err := db.UpdateNodeStatus(nodeID, "ONLINE"); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    log.Printf("🟢 Node %s set to ONLINE", nodeID)
    c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Node %s is now ONLINE", nodeID), "status": "ONLINE"})
}

// ─── Health & Stats ────────────────────────────────────

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
```

---

## Upload Flow Visualization

```
React  ─── POST /api/files/upload ──►  Handler.UploadFile()
                                            │
                                            ▼
                                      Read multipart file
                                            │
                                            ▼
                                      SHA-256(file) → checksum
                                            │
                                            ▼
                                      SplitData(file, 4MB)
                                            │
                                    ┌───────┼───────┐
                                    │       │       │
                                 Chunk 0  Chunk 1  Chunk N
                                    │       │       │
                                    ▼       ▼       ▼
                              RS Encode  RS Encode  RS Encode
                                    │       │       │
                              D1 D2 D3 P  ...      ...
                                    │
                              ┌─────┼─────┬─────┐
                              ▼     ▼     ▼     ▼
                           gRPC  gRPC  gRPC  gRPC
                              │     │     │     │
                           Node1 Node2 Node3 Node4
                              │     │     │     │
                            Disk  Disk  Disk  Disk
```

---

## Verification

```bash
cd coordinator
go build ./...
# Should compile cleanly

# Test with curl (after Docker Compose is up in Step 09):
# Upload
curl -X POST http://localhost:8080/api/files/upload \
    -F "file=@testfile.txt"

# List files
curl http://localhost:8080/api/files

# List nodes
curl http://localhost:8080/api/nodes

# Stop a node
curl -X POST http://localhost:8080/api/nodes/node-2/stop
```

---

## What This Step Achieves
- Complete REST API with all endpoints
- Database query layer for files, chunks, shards, and nodes
- Shard Manager that distributes shards to nodes via gRPC streaming
- Upload flow: file → SHA-256 → chunk → RS encode → gRPC store → metadata to PostgreSQL
- Node start/stop simulation via REST endpoints
