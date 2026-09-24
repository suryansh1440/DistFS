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
	var lastHeartbeat sql.NullTime
	err := DB.QueryRow(`
		SELECT n.id, n.name, n.grpc_address, n.status, n.total_storage,
		       COALESCE(SUM(s.shard_size), 0) AS used_storage,
		       COUNT(s.id) AS shard_count,
		       n.last_heartbeat, n.created_at, n.updated_at
		FROM nodes n
		LEFT JOIN shards s ON s.node_id = n.id AND s.status = 'STORED'
		WHERE n.id = $1
		GROUP BY n.id, n.name, n.grpc_address, n.status, n.total_storage, n.last_heartbeat, n.created_at, n.updated_at`, nodeID).Scan(
		&n.ID, &n.Name, &n.GRPCAddress, &n.Status, &n.TotalStorage, &n.UsedStorage,
		&n.ShardCount, &lastHeartbeat, &n.CreatedAt, &n.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}
	if lastHeartbeat.Valid {
		n.LastHeartbeat = lastHeartbeat.Time
	}
	return n, err
}

func ListNodes() ([]models.Node, error) {
	rows, err := DB.Query(`
		SELECT n.id, n.name, n.grpc_address, n.status, n.total_storage,
		       COALESCE(SUM(s.shard_size), 0) AS used_storage,
		       COUNT(s.id) AS shard_count,
		       n.last_heartbeat, n.created_at, n.updated_at
		FROM nodes n
		LEFT JOIN shards s ON s.node_id = n.id AND s.status = 'STORED'
		GROUP BY n.id, n.name, n.grpc_address, n.status, n.total_storage, n.last_heartbeat, n.created_at, n.updated_at
		ORDER BY n.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []models.Node
	for rows.Next() {
		var n models.Node
		var lastHeartbeat sql.NullTime
		err := rows.Scan(&n.ID, &n.Name, &n.GRPCAddress, &n.Status, &n.TotalStorage, &n.UsedStorage,
			&n.ShardCount, &lastHeartbeat, &n.CreatedAt, &n.UpdatedAt)
		if err != nil {
			return nil, err
		}
		if lastHeartbeat.Valid {
			n.LastHeartbeat = lastHeartbeat.Time
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
		SELECT n.id, n.name, n.grpc_address, n.status, n.total_storage,
		       COALESCE(SUM(s.shard_size), 0) AS used_storage,
		       COUNT(s.id) AS shard_count,
		       n.last_heartbeat, n.created_at, n.updated_at
		FROM nodes n
		LEFT JOIN shards s ON s.node_id = n.id AND s.status = 'STORED'
		WHERE n.status = 'ONLINE'
		GROUP BY n.id, n.name, n.grpc_address, n.status, n.total_storage, n.last_heartbeat, n.created_at, n.updated_at
		ORDER BY n.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []models.Node
	for rows.Next() {
		var n models.Node
		var lastHeartbeat sql.NullTime
		err := rows.Scan(&n.ID, &n.Name, &n.GRPCAddress, &n.Status, &n.TotalStorage, &n.UsedStorage,
			&n.ShardCount, &lastHeartbeat, &n.CreatedAt, &n.UpdatedAt)
		if err != nil {
			return nil, err
		}
		if lastHeartbeat.Valid {
			n.LastHeartbeat = lastHeartbeat.Time
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}
