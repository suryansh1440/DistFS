-- ══════════════════════════════════════════════════════════
-- Migration 003: Chunks
-- ══════════════════════════════════════════════════════════
-- Each file is split into N chunks (default 4 MB each).
-- Each chunk is independently Reed-Solomon encoded into K+M shards.

CREATE TABLE IF NOT EXISTS chunks (
    id              VARCHAR(36) PRIMARY KEY,
    file_id         VARCHAR(36) NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    chunk_index     INT NOT NULL,                -- 0-based position within the file
    chunk_size      BIGINT NOT NULL,             -- actual bytes (last chunk may be smaller)
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    UNIQUE(file_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS idx_chunks_file_id ON chunks(file_id);
