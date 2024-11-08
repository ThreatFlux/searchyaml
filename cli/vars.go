package cli

import (
	"flag"
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
