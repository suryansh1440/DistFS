-- ══════════════════════════════════════════════════════════
-- Migration 001: Storage Nodes
-- ══════════════════════════════════════════════════════════
-- Each row represents one of the 4 storage nodes in the cluster.
-- Status is controlled by the coordinator (dashboard stop/start).
-- Actual shard data lives on each node's local disk, NOT in this table.

CREATE TABLE IF NOT EXISTS nodes (
    id              VARCHAR(36) PRIMARY KEY,
    name            VARCHAR(50) NOT NULL UNIQUE,
    grpc_address    VARCHAR(255) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'ONLINE',   -- ONLINE | OFFLINE
    total_storage   BIGINT NOT NULL DEFAULT 0,                -- capacity in bytes
    used_storage    BIGINT NOT NULL DEFAULT 0,                -- consumed bytes
    shard_count     INT NOT NULL DEFAULT 0,
    last_heartbeat  TIMESTAMP WITH TIME ZONE,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Seed the 4 storage nodes with 10 GB capacity each.
-- gRPC addresses use Docker Compose service names for internal networking.
INSERT INTO nodes (id, name, grpc_address, status, total_storage) VALUES
    ('node-1', 'node-1', 'storage-node-1:50051', 'ONLINE', 10737418240),
    ('node-2', 'node-2', 'storage-node-2:50052', 'ONLINE', 10737418240),
    ('node-3', 'node-3', 'storage-node-3:50053', 'ONLINE', 10737418240),
    ('node-4', 'node-4', 'storage-node-4:50054', 'ONLINE', 10737418240)
ON CONFLICT (id) DO NOTHING;
