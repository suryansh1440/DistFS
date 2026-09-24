# Step 07 — Node Manager & Health / Simulation

## Goal
Build the Node Manager in the coordinator that handles node health tracking and the simulation logic for stopping/starting nodes. This step also updates the coordinator's `main.go` entry point.

---

## What "Stop/Start" Actually Does

Your system uses **soft simulation** — the coordinator simply marks a node as OFFLINE in PostgreSQL. When a node is OFFLINE:

1. **Upload**: The coordinator refuses to send shards to that node (upload fails if all nodes aren't online)
2. **Download**: The coordinator skips that node and uses Reed-Solomon reconstruction
3. **gRPC**: The actual gRPC server on the node is still running (the Docker container is still up), but the coordinator *chooses not to contact it*

This is simpler and more reliable than actually killing Docker containers from the React dashboard.

```
Dashboard clicks "STOP Node 2"
         │
         ▼
POST /api/nodes/node-2/stop
         │
         ▼
Coordinator updates PostgreSQL:
    nodes SET status = 'OFFLINE' WHERE id = 'node-2'
         │
         ▼
All future operations check node.Status before contacting it.
Node 2's gRPC server is still running, just ignored.
```

---

## Files to Create

### `coordinator/internal/nodemanager/manager.go`

```go
package nodemanager

import (
    "fmt"
    "log"
    "sync"

    "github.com/surya/dist-file-storage/coordinator/internal/db"
    "github.com/surya/dist-file-storage/coordinator/internal/models"
    "github.com/surya/dist-file-storage/coordinator/internal/shardmanager"
)

// NodeManager tracks node states and provides cluster health information.
type NodeManager struct {
    shardMgr *shardmanager.ShardManager
    mu       sync.RWMutex
}

func NewNodeManager(sm *shardmanager.ShardManager) *NodeManager {
    return &NodeManager{
        shardMgr: sm,
    }
}

// IsNodeOnline checks if a specific node is marked ONLINE in the database.
func (nm *NodeManager) IsNodeOnline(nodeID string) (bool, error) {
    node, err := db.GetNode(nodeID)
    if err != nil {
        return false, err
    }
    return node.Status == "ONLINE", nil
}

// GetClusterHealth returns a summary of cluster health.
type ClusterHealth struct {
    TotalNodes    int            `json:"total_nodes"`
    OnlineNodes   int            `json:"online_nodes"`
    OfflineNodes  int            `json:"offline_nodes"`
    NodeStatuses  map[string]string `json:"node_statuses"` // nodeID → status
    CanUpload     bool           `json:"can_upload"`       // need all 4 nodes online
    CanDownload   bool           `json:"can_download"`     // need at least 3 nodes online
    HealthStatus  string         `json:"health_status"`    // HEALTHY | DEGRADED | CRITICAL
}

func (nm *NodeManager) GetClusterHealth() (*ClusterHealth, error) {
    nm.mu.RLock()
    defer nm.mu.RUnlock()

    nodes, err := db.ListNodes()
    if err != nil {
        return nil, err
    }

    health := &ClusterHealth{
        TotalNodes:   len(nodes),
        NodeStatuses: make(map[string]string),
    }

    for _, n := range nodes {
        health.NodeStatuses[n.ID] = n.Status
        if n.Status == "ONLINE" {
            health.OnlineNodes++
        } else {
            health.OfflineNodes++
        }
    }

    // Upload requires all nodes to be online (so every shard can be stored)
    health.CanUpload = health.OnlineNodes == health.TotalNodes

    // Download requires at least K (3) nodes to be online (for RS reconstruction)
    health.CanDownload = health.OnlineNodes >= 3

    // Determine overall health
    switch {
    case health.OnlineNodes == health.TotalNodes:
        health.HealthStatus = "HEALTHY"
    case health.OnlineNodes >= 3:
        health.HealthStatus = "DEGRADED"
    default:
        health.HealthStatus = "CRITICAL"
    }

    return health, nil
}

// StopNode marks a node as OFFLINE.
func (nm *NodeManager) StopNode(nodeID string) error {
    nm.mu.Lock()
    defer nm.mu.Unlock()

    node, err := db.GetNode(nodeID)
    if err != nil {
        return fmt.Errorf("node not found: %s", nodeID)
    }

    if node.Status == "OFFLINE" {
        return fmt.Errorf("node %s is already OFFLINE", nodeID)
    }

    if err := db.UpdateNodeStatus(nodeID, "OFFLINE"); err != nil {
        return err
    }

    log.Printf("🔴 Node %s → OFFLINE", nodeID)
    return nil
}

// StartNode marks a node as ONLINE and verifies gRPC connectivity.
func (nm *NodeManager) StartNode(nodeID string) error {
    nm.mu.Lock()
    defer nm.mu.Unlock()

    node, err := db.GetNode(nodeID)
    if err != nil {
        return fmt.Errorf("node not found: %s", nodeID)
    }

    if node.Status == "ONLINE" {
        return fmt.Errorf("node %s is already ONLINE", nodeID)
    }

    // Verify the node is actually reachable via gRPC
    healthResp, err := nm.shardMgr.HealthCheckNode(nodeID)
    if err != nil {
        log.Printf("⚠ Node %s health check failed: %v", nodeID, err)
        // Still mark it online — it might recover
    } else {
        // Update storage stats from the health check
        db.UpdateNodeStats(nodeID, healthResp.UsedStorage, int(healthResp.ShardCount))
    }

    if err := db.UpdateNodeStatus(nodeID, "ONLINE"); err != nil {
        return err
    }

    log.Printf("🟢 Node %s → ONLINE", nodeID)
    return nil
}

// RefreshNodeStats queries each online node's health and updates the database.
func (nm *NodeManager) RefreshNodeStats() {
    nodes, err := db.ListNodes()
    if err != nil {
        log.Printf("⚠ Failed to list nodes for refresh: %v", err)
        return
    }

    for _, node := range nodes {
        if node.Status != "ONLINE" {
            continue
        }

        healthResp, err := nm.shardMgr.HealthCheckNode(node.ID)
        if err != nil {
            log.Printf("⚠ Health check failed for %s: %v", node.ID, err)
            continue
        }

        db.UpdateNodeStats(node.ID, healthResp.UsedStorage, int(healthResp.ShardCount))
    }
}

// GetAvailableNodesForChunk returns which nodes are online for shard retrieval.
// Returns: map[shardIndex] → nodeID (only for available nodes)
func (nm *NodeManager) GetAvailableNodesForChunk(shards []models.Shard) (available map[int]string, missing map[int]string) {
    available = make(map[int]string)
    missing = make(map[int]string)

    for _, s := range shards {
        node, err := db.GetNode(s.NodeID)
        if err != nil || node.Status != "ONLINE" {
            missing[s.ShardIndex] = s.NodeID
        } else {
            available[s.ShardIndex] = s.NodeID
        }
    }

    return available, missing
}
```

---

### `coordinator/cmd/server/main.go` (complete entry point)

```go
package main

import (
    "log"
    "os"

    "github.com/joho/godotenv"

    "github.com/surya/dist-file-storage/coordinator/internal/api"
    "github.com/surya/dist-file-storage/coordinator/internal/db"
    "github.com/surya/dist-file-storage/coordinator/internal/erasure"
    "github.com/surya/dist-file-storage/coordinator/internal/shardmanager"
)

func main() {
    log.Println("╔══════════════════════════════════════════╗")
    log.Println("║   Distributed File Storage Coordinator   ║")
    log.Println("╚══════════════════════════════════════════╝")

    // Load .env file (optional — Docker Compose injects env vars)
    godotenv.Load()

    // 1. Connect to PostgreSQL
    if err := db.InitDB(); err != nil {
        log.Fatalf("Database connection failed: %v", err)
    }

    // 2. Run migrations
    if err := db.RunMigrations(); err != nil {
        log.Fatalf("Migrations failed: %v", err)
    }

    // 3. Initialize Reed-Solomon encoder
    rsConfig := erasure.DefaultConfig() // 3 data + 1 parity
    encoder, err := erasure.NewEncoder(rsConfig)
    if err != nil {
        log.Fatalf("Failed to create RS encoder: %v", err)
    }
    log.Printf("✓ Reed-Solomon: %d data + %d parity shards", rsConfig.DataShards, rsConfig.ParityShards)

    // 4. Initialize Shard Manager (gRPC connections to storage nodes)
    sm := shardmanager.NewShardManager()
    if err := sm.ConnectToNodes(); err != nil {
        log.Printf("⚠ Some node connections failed: %v", err)
    }

    // 5. Start REST API
    port := getEnv("COORDINATOR_PORT", "8080")
    router := api.SetupRouter(sm, encoder)

    log.Printf("✓ Coordinator listening on :%s", port)
    if err := router.Run(":" + port); err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
}

func getEnv(key, fallback string) string {
    if val := os.Getenv(key); val != "" {
        return val
    }
    return fallback
}
```

---

## Node Status Flow

```
              ┌──────────┐
              │  ONLINE  │
              └────┬─────┘
                   │
          POST /nodes/:id/stop
                   │
                   ▼
              ┌──────────┐
              │  OFFLINE │
              └────┬─────┘
                   │
          POST /nodes/:id/start
                   │
                   ▼
              ┌──────────┐
              │  ONLINE  │
              └──────────┘
```

## Cluster Health Matrix

| Online Nodes | Health | Can Upload? | Can Download? |
|:---:|:---:|:---:|:---:|
| 4/4 | ✅ HEALTHY | ✅ Yes | ✅ Yes |
| 3/4 | ⚠️ DEGRADED | ❌ No | ✅ Yes (with RS reconstruction) |
| 2/4 | 🔴 CRITICAL | ❌ No | ❌ No |
| 1/4 | 🔴 CRITICAL | ❌ No | ❌ No |

---

## Verification

```bash
cd coordinator
go build ./...
# Should compile with all modules

# The coordinator now starts up, connects to DB, runs migrations,
# connects to storage nodes, and serves REST API.
```

---

## What This Step Achieves
- Node Manager with stop/start simulation logic
- Cluster health tracking (HEALTHY / DEGRADED / CRITICAL)
- Upload/download feasibility checks based on node availability
- Complete coordinator entry point (`main.go`) that wires everything together
- Ready for the download/reconstruction flow (Step 08)
