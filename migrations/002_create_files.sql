-- ══════════════════════════════════════════════════════════
-- Migration 002: Files
-- ══════════════════════════════════════════════════════════
-- Each row is one uploaded file.
-- Stores the original file's SHA-256 checksum for download validation.
-- The actual file bytes are split into chunks → shards → stored on nodes.

CREATE TABLE IF NOT EXISTS files (
    id                VARCHAR(36) PRIMARY KEY,
    filename          VARCHAR(500) NOT NULL,
    file_size         BIGINT NOT NULL,                         -- original size in bytes
    original_checksum VARCHAR(64) NOT NULL,                    -- SHA-256 hex digest
    total_chunks      INT NOT NULL,
    data_shards       INT NOT NULL DEFAULT 3,                  -- K (Reed-Solomon)
    parity_shards     INT NOT NULL DEFAULT 1,                  -- M (Reed-Solomon)
    chunk_size        INT NOT NULL DEFAULT 4194304,            -- 4 MB
    status            VARCHAR(20) NOT NULL DEFAULT 'UPLOADING',
                      -- UPLOADING → HEALTHY → DEGRADED → FAILED
    created_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_files_status ON files(status);
CREATE INDEX IF NOT EXISTS idx_files_filename ON files(filename);
