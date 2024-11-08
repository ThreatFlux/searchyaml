package main

import (
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"searchYAML/storage"
	"time"
)

var InitialSize = flag.Int64("size", 32<<20, "Initial file size in bytes")

// Configuration flags
var (
	Debug    = flag.Bool("debug", false, "Enable debug logging")
	Port     = flag.String("port", ":8080", "Server port")
	DataFile = flag.String("data", "data.yaml", "Data file path")

	MaxSize      = flag.Int64("maxsize", 512<<20, "Maximum file size in bytes")
	SyncInterval = flag.Duration("sync", time.Minute, "Sync interval")
)

func main() {
	flag.Parse()

	if !*Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	// Initialize store with options
	opts := storage.StoreOptions{
		InitialSize:  *InitialSize,
		MaxSize:      *MaxSize,
		SyncInterval: *SyncInterval,
		Debug:        *Debug,
	}

	store, err := storage.NewStore(*DataFile, opts)
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}
	defer func(store *storage.Store) {
		err := store.Close()
		if err != nil {
			log.Fatalf("Failed to close store: %v", err)
		}
	}(store)

	// Create default indexes
	if err := createDefaultIndexes(store); err != nil {
		log.Fatalf("Failed to create indexes: %v", err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	if *Debug {
		r.Use(gin.Logger())
	}

	// CRUD endpoints
	data := r.Group("/data")
	{
		data.GET("/:key", handleGet(store))
		data.POST("/:key", handleSet(store))
		data.DELETE("/:key", handleDelete(store))
	}

	// Search endpoints
	search := r.Group("/search")
	{
		search.POST("/text", handleTextSearch(store))
		search.POST("/vector", handleVectorSearch(store))
		search.POST("/combined", handleCombinedSearch(store))
	}

	// Index management endpoints
	index := r.Group("/index")
	{
		index.POST("/create", handleCreateIndex(store))
		index.DELETE("/remove", handleRemoveIndex(store))
	}

	// Admin endpoints
	admin := r.Group("/admin")
	{
		admin.POST("/sync", handleSync(store))
		admin.GET("/stats", handleStats(store))
	}

	log.Printf("Starting server on %s", *Port)
	if err := r.Run(*Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// Handler functions

func handleGet(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		entry, exists := store.Get(key)
		if !exists {
			c.JSON(404, gin.H{"error": "key not found"})
			return
		}

		if c.GetHeader("Accept") == "application/x-yaml" {
			c.YAML(200, gin.H{key: entry})
		} else {
			c.JSON(200, gin.H{key: entry})
		}
	}
}

func handleSet(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		var value interface{}

		// Parse request body based on content type
		if err := parseRequestBody(c, &value); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		// Handle TTL if specified
		if ttl := c.GetHeader("X-TTL"); ttl != "" {
			duration, err := time.ParseDuration(ttl)
			if err != nil {
				c.JSON(400, gin.H{"error": "invalid TTL format"})
				return
			}
			if err := store.SetWithTTL(key, value, duration); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
		} else {
			if err := store.Set(key, value); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
		}

		c.JSON(200, gin.H{"status": "ok"})
	}
}

func handleDelete(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		store.Delete(c.Param("key"))
		c.JSON(200, gin.H{"status": "ok"})
	}
}

func handleTextSearch(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var query struct {
			Text       string  `json:"text" binding:"required"`
			MaxResults int     `json:"max_results"`
			MinScore   float64 `json:"min_score"`
		}

		if err := c.ShouldBindJSON(&query); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		searchQuery := storage.SearchQuery{
			Text:       query.Text,
			MaxResults: query.MaxResults,
			MinScore:   query.MinScore,
		}

		results, err := store.Search(searchQuery)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, results)
	}
}

func handleVectorSearch(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var query struct {
			Vector     []float32 `json:"vector" binding:"required"`
			MaxResults int       `json:"max_results"`
			MinScore   float64   `json:"min_score"`
		}

		if err := c.ShouldBindJSON(&query); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		searchQuery := storage.SearchQuery{
			Vector:     query.Vector,
			MaxResults: query.MaxResults,
			MinScore:   query.MinScore,
		}

		results, err := store.Search(searchQuery)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, results)
	}
}

func handleCombinedSearch(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var query storage.SearchQuery
		if err := c.ShouldBindJSON(&query); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		results, err := store.Search(query)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, results)
	}
}

func handleSync(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := store.Sync(); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	}
}

func handleStats(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats := store.GetStats()
		if c.GetHeader("Accept") == "application/x-yaml" {
			c.YAML(200, stats)
		} else {
			c.JSON(200, stats)
		}
	}
}

// Helper functions

func createDefaultIndexes(store *storage.Store) error {
	defaults := []struct {
		field string
		type_ string
	}{
		{"title", "text"},
		{"description", "text"},
		{"tags", "text"},
		{"embedding", "vector"},
	}

	for _, idx := range defaults {
		if err := store.CreateIndex(idx.field, idx.type_); err != nil {
			return fmt.Errorf("failed to create index %s: %v", idx.field, err)
		}
	}

	return nil
}

func handleCreateIndex(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Field string `json:"field" binding:"required"`
			Type  string `json:"type" binding:"required"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		if err := store.CreateIndex(request.Field, request.Type); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"status": "ok"})
	}
}

func handleRemoveIndex(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Field string `json:"field" binding:"required"`
			Type  string `json:"type" binding:"required"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		// We'll need to add RemoveIndex to the Store type
		if err := store.RemoveIndex(request.Field, request.Type); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"status": "ok"})
	}
}

func parseRequestBody(c *gin.Context, value interface{}) error {
	switch c.GetHeader("Content-Type") {
	case "application/x-yaml":
		return c.BindYAML(value)
	case "application/json", "":
		return c.BindJSON(value)
	default:
		return fmt.Errorf("unsupported content type")
	}
}
