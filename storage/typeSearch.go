package storage

import (
	"sync"
	"time"
)

// StoreStats tracks operational statistics for monitoring.
type StoreStats struct {
	sync.Mutex
	// Basic Operations
	Reads        uint64    `json:"reads" yaml:"reads"`
	Writes       uint64    `json:"writes" yaml:"writes"`
	Deletes      uint64    `json:"deletes" yaml:"deletes"`
	SyncCount    uint64    `json:"sync_count" yaml:"sync_count"`
	LastSyncTime time.Time `json:"last_sync_time" yaml:"last_sync_time"`

	// Storage Stats
	DataSize     int64  `json:"data_size" yaml:"data_size"`         // Current size of YAML data
	FileSize     int64  `json:"file_size" yaml:"file_size"`         // Total size of mmap file
	EntryCount   uint64 `json:"entry_count" yaml:"entry_count"`     // Number of active entries
	ExpiredCount uint64 `json:"expired_count" yaml:"expired_count"` // Number of expired entries

	// Index Stats
	IndexStats struct {
		TextIndexes struct {
			Count      int `json:"count" yaml:"count"`
			EntryCount int `json:"entry_count" yaml:"entry_count"`
		} `json:"text_indexes" yaml:"text_indexes"`
		VectorIndexes struct {
			Count      int `json:"count" yaml:"count"`
			EntryCount int `json:"entry_count" yaml:"entry_count"`
		} `json:"vector_indexes" yaml:"vector_indexes"`
		BTreeIndexes struct {
			Count      int `json:"count" yaml:"count"`
			EntryCount int `json:"entry_count" yaml:"entry_count"`
		} `json:"btree_indexes" yaml:"btree_indexes"`
	} `json:"index_stats" yaml:"index_stats"`

	// Performance Stats
	PerformanceStats struct {
		AvgReadLatency  float64   `json:"avg_read_latency" yaml:"avg_read_latency"`   // in milliseconds
		AvgWriteLatency float64   `json:"avg_write_latency" yaml:"avg_write_latency"` // in milliseconds
		AvgSyncLatency  float64   `json:"avg_sync_latency" yaml:"avg_sync_latency"`   // in milliseconds
		LastGC          time.Time `json:"last_gc" yaml:"last_gc"`                     // Last time expired entries were cleaned
	} `json:"performance_stats" yaml:"performance_stats"`
}
