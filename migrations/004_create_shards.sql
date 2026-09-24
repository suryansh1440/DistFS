-- ══════════════════════════════════════════════════════════
-- Migration 004: Shards
-- ══════════════════════════════════════════════════════════
-- Each chunk produces K data shards + M parity shards.
-- This table tracks WHERE each shard is stored (which node)
-- and its checksum for integrity verification.
--
-- IMPORTANT: The actual shard BYTES live on the storage node's
-- local disk at /data/shards/<file_id>/<chunk_id>_<shard_index>.shard
-- This table only stores metadata.

CREATE TABLE IF NOT EXISTS shards (
    id              VARCHAR(36) PRIMARY KEY,
    chunk_id        VARCHAR(36) NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    file_id         VARCHAR(36) NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    shard_index     INT NOT NULL,                -- 0..K-1 = data, K..K+M-1 = parity
    shard_type      VARCHAR(10) NOT NULL,        -- DATA | PARITY
    node_id         VARCHAR(36) NOT NULL REFERENCES nodes(id),
    shard_size      BIGINT NOT NULL,
    checksum        VARCHAR(64) NOT NULL,        -- SHA-256 of this individual shard
    status          VARCHAR(20) NOT NULL DEFAULT 'STORED',
                    -- STORED | MISSING | CORRUPTED
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    UNIQUE(chunk_id, shard_index)
);

CREATE INDEX IF NOT EXISTS idx_shards_chunk_id ON shards(chunk_id);
CREATE INDEX IF NOT EXISTS idx_shards_file_id ON shards(file_id);
CREATE INDEX IF NOT EXISTS idx_shards_node_id ON shards(node_id);
