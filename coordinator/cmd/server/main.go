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
	log.Println("==========================================")
	log.Println("   Distributed File Storage Coordinator   ")
	log.Println("==========================================")

	// Load .env file (optional; ignore if not present)
	_ = godotenv.Load()

	// 1. Connect to PostgreSQL
	if err := db.InitDB(); err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}

	// 2. Run migrations to initialize tables and seed storage nodes
	if err := db.RunMigrations(); err != nil {
		log.Fatalf("Migrations failed: %v", err)
	}

	// 3. Initialize Reed-Solomon encoder (3 data + 1 parity for 4 nodes)
	rsConfig := erasure.DefaultConfig()
	encoder, err := erasure.NewEncoder(rsConfig)
	if err != nil {
		log.Fatalf("Failed to create RS encoder: %v", err)
	}
	log.Printf("[OK] Reed-Solomon configured: %d data + %d parity shards", rsConfig.DataShards, rsConfig.ParityShards)

	// 4. Initialize Shard Manager (gRPC connections to storage nodes)
	sm := shardmanager.NewShardManager()
	if err := sm.ConnectToNodes(); err != nil {
		log.Printf("[WARN] Initial node connections: %v", err)
	}

	// 5. Start REST API server
	port := getEnv("COORDINATOR_PORT", "8080")
	router := api.SetupRouter(sm, encoder)

	log.Printf("[OK] Coordinator listening on :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start coordinator HTTP server: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
