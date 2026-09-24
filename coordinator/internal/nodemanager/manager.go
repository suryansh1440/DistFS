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

// ClusterHealth returns a summary of cluster health.
type ClusterHealth struct {
	TotalNodes   int               `json:"total_nodes"`
	OnlineNodes  int               `json:"online_nodes"`
	OfflineNodes int               `json:"offline_nodes"`
	NodeStatuses map[string]string `json:"node_statuses"` // nodeID -> status
	CanUpload    bool              `json:"can_upload"`     // need all 4 nodes online
	CanDownload  bool              `json:"can_download"`   // need at least 3 nodes online
	HealthStatus string            `json:"health_status"`  // HEALTHY | DEGRADED | CRITICAL
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

	log.Printf("Node %s -> OFFLINE", nodeID)
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
		log.Printf("[WARN] Node %s health check failed: %v", nodeID, err)
		// Still mark it online — it might recover or start up
	} else {
		// Update storage stats from the health check
		_ = db.UpdateNodeStats(nodeID, healthResp.UsedStorage, int(healthResp.ShardCount))
	}

	if err := db.UpdateNodeStatus(nodeID, "ONLINE"); err != nil {
		return err
	}

	log.Printf("Node %s -> ONLINE", nodeID)
	return nil
}

// RefreshNodeStats queries each online node's health and updates the database.
func (nm *NodeManager) RefreshNodeStats() {
	nodes, err := db.ListNodes()
	if err != nil {
		log.Printf("[WARN] Failed to list nodes for refresh: %v", err)
		return
	}

	for _, node := range nodes {
		if node.Status != "ONLINE" {
			continue
		}

		healthResp, err := nm.shardMgr.HealthCheckNode(node.ID)
		if err != nil {
			log.Printf("[WARN] Health check failed for %s: %v", node.ID, err)
			continue
		}

		_ = db.UpdateNodeStats(node.ID, healthResp.UsedStorage, int(healthResp.ShardCount))
	}
}

// GetAvailableNodesForChunk returns which nodes are online for shard retrieval.
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
