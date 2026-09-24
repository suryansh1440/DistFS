package api

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/surya/dist-file-storage/coordinator/internal/erasure"
	"github.com/surya/dist-file-storage/coordinator/internal/shardmanager"
)

func SetupRouter(sm *shardmanager.ShardManager, enc *erasure.Encoder) *gin.Engine {
	r := gin.Default()

	// CORS configuration for React dashboard
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept"},
		ExposeHeaders:    []string{"Content-Disposition", "X-Original-Checksum", "X-Reconstructed-Checksum", "X-Reconstructed", "X-Missing-Shards", "X-Recovered-Shards"},
	}))

	handler := NewHandler(sm, enc)

	api := r.Group("/api")
	{
		// Health & Stats
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
