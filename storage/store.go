package storage

import (
	"fmt"
	"github.com/edsrzf/mmap-go"
	"log"
	"os"
	"sync"
	"time"
)

// Entry represents a single value in the store with metadata
type Entry struct {
	Value     interface{} `yaml:"value"`
	Timestamp int64       `yaml:"timestamp,omitempty"`
	TTL       int64       `yaml:"ttl,omitempty"`
}

// Store represents an enhanced memory-mapped key-value store
type Store struct {
	sync.RWMutex
	mm       mmap.MMap
	filepath string
	data     map[string]*Entry
	dirty    bool
	stats    StoreStats
	encoder  *FastYAMLEncoder
	indexes  *IndexManager
}

// StoreOptions configures the store initialization
type StoreOptions struct {
	InitialSize  int64
	MaxSize      int64
	SyncInterval time.Duration
	Debug        bool
}

var DefaultOptions = StoreOptions{
	InitialSize:  32 << 20,  // 32MB
	MaxSize:      512 << 20, // 512MB
	SyncInterval: time.Minute,
	Debug:        false,
}

// NewStore creates a new memory-mapped store with the given options
func NewStore(filepath string, opts StoreOptions) (*Store, error) {
	file, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %v", err)
	}

	if info.Size() < opts.InitialSize {
		if err := file.Truncate(opts.InitialSize); err != nil {
			return nil, fmt.Errorf("failed to truncate file: %v", err)
		}
	}

	mm, err := mmap.Map(file, mmap.RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to map file: %v", err)
	}

	store := &Store{
		mm:       mm,
		filepath: filepath,
		data:     make(map[string]*Entry, 1000),
		encoder:  NewFastYAMLEncoder(),
		indexes:  NewIndexManager(),
	}

	// Initialize file size stat
	store.stats.FileSize = info.Size()

	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("error loading existing data: %v", err)
	}

	go store.periodicSync(opts.SyncInterval)

	return store, nil
}

// CRUD Operations with performance tracking

func (s *Store) Get(key string) (*Entry, bool) {
	start := time.Now()
	defer func() {
		s.updateReadStats(time.Since(start))
	}()

	s.RLock()
	defer s.RUnlock()

	entry, exists := s.data[key]
	if exists {
		if entry.TTL > 0 && time.Now().Unix() > entry.Timestamp+entry.TTL {
			go s.Delete(key) // Async cleanup
			return nil, false
		}
	}

	return entry, exists
}

func (s *Store) Set(key string, value interface{}) error {
	start := time.Now()
	defer func() {
		s.updateWriteStats(time.Since(start))
	}()

	s.Lock()
	defer s.Unlock()

	entry := &Entry{
		Value:     value,
		Timestamp: time.Now().Unix(),
	}

	s.data[key] = entry
	s.dirty = true

	if err := s.indexes.Update(key, value); err != nil {
		return fmt.Errorf("failed to update indexes: %v", err)
	}

	return nil
}

// periodicSync runs a background goroutine that periodically syncs data to disk
func (s *Store) periodicSync(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				start := time.Now()
				if err := s.sync(); err != nil {
					log.Printf("Error during periodic sync: %v", err)
				}
				// Update sync latency statistics
				s.updateSyncStats(time.Since(start))

				// Perform garbage collection of expired entries
				s.gcExpiredEntries()
			}
		}
	}()
}

// sync writes the current data to the memory-mapped file with optimized YAML encoding
func (s *Store) sync() error {
	if !s.dirty {
		return nil // Skip sync if no changes
	}

	// Create a map without expired entries
	cleanData := make(map[string]*Entry)
	now := time.Now().Unix()

	for key, entry := range s.data {
		if entry.TTL == 0 || now <= entry.Timestamp+entry.TTL {
			cleanData[key] = entry
		}
	}

	// Encode the data
	data, err := s.encoder.Encode(cleanData)
	if err != nil {
		return fmt.Errorf("failed to encode data: %v", err)
	}

	// Check if we need to grow the file
	if len(data) > len(s.mm) {
		if err := s.growFile(int64(len(data))); err != nil {
			return fmt.Errorf("failed to grow file: %v", err)
		}
	}

	// Write YAML data and pad with zeros
	copy(s.mm, data)
	for i := len(data); i < len(s.mm); i++ {
		s.mm[i] = 0
	}

	if err := s.mm.Flush(); err != nil {
		return fmt.Errorf("failed to flush to disk: %v", err)
	}

	s.dirty = false
	s.updateStats(int64(len(data)))

	return nil
}

// resize grows or shrinks the memory-mapped file
func (s *Store) resize(newSize int64) error {
	s.Lock()
	defer s.Unlock()

	// Save current data
	if err := s.sync(); err != nil {
		return fmt.Errorf("failed to sync before resize: %v", err)
	}

	// Unmap current file
	if err := s.mm.Unmap(); err != nil {
		return fmt.Errorf("failed to unmap: %v", err)
	}

	// Resize file
	file, err := os.OpenFile(s.filepath, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for resize: %v", err)
	}
	defer file.Close()

	if err := file.Truncate(newSize); err != nil {
		return fmt.Errorf("failed to truncate: %v", err)
	}

	// Remap file
	mm, err := mmap.Map(file, mmap.RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to remap: %v", err)
	}

	s.mm = mm
	return nil
}

// growFile increases the file size to accommodate new data
func (s *Store) growFile(requiredSize int64) error {
	currentSize := int64(len(s.mm))
	newSize := currentSize * 2

	for newSize < requiredSize {
		newSize *= 2
	}

	return s.resize(newSize)
}

func (s *Store) SetWithTTL(key string, value interface{}, ttl time.Duration) error {
	s.Lock()
	defer s.Unlock()

	entry := &Entry{
		Value:     value,
		Timestamp: time.Now().Unix(),
		TTL:       int64(ttl.Seconds()),
	}

	s.data[key] = entry
	s.dirty = true

	if err := s.indexes.Update(key, value); err != nil {
		return fmt.Errorf("failed to update indexes: %v", err)
	}

	s.stats.Lock()
	s.stats.Writes++
	s.stats.Unlock()

	return nil
}

func (s *Store) Delete(key string) {
	s.Lock()
	defer s.Unlock()

	if _, exists := s.data[key]; exists {
		delete(s.data, key)
		s.dirty = true
		s.indexes.Remove(key)

		s.stats.Lock()
		s.stats.Deletes++
		s.stats.Unlock()
	}
}

// Close ensures all data is synced and resources are released
func (s *Store) Close() error {
	s.Lock()
	defer s.Unlock()

	if err := s.sync(); err != nil {
		return fmt.Errorf("failed to sync on close: %v", err)
	}

	if err := s.mm.Unmap(); err != nil {
		return fmt.Errorf("failed to unmap on close: %v", err)
	}

	return nil
}

// Sync forces a sync to disk
func (s *Store) Sync() error {
	return s.sync()
}

// CreateIndex creates a new index of the specified type
func (s *Store) CreateIndex(field string, indexType string) error {
	return s.indexes.AddIndex(field, indexType)
}

// AddToIndex adds or updates a value in the specified index
func (s *Store) AddToIndex(field string, key string, value interface{}) error {
	return s.indexes.Update(key, map[string]interface{}{field: value})
}

// RemoveIndex removes an index of the specified type
func (s *Store) RemoveIndex(field string, indexType string) error {
	return s.indexes.RemoveIndex(field, indexType)
}

// load reads the YAML data from the memory-mapped file with optimized parsing
func (s *Store) load() error {
	s.Lock()
	defer s.Unlock()

	// Find valid YAML content
	size := s.findContentSize()
	if size == 0 {
		return nil // Empty file is valid
	}

	// Create a temporary map to hold the data
	var tempData map[string]*Entry

	// Use the encoder to decode the data
	if err := s.encoder.Decode(s.mm[:size], &tempData); err != nil {
		return fmt.Errorf("failed to decode YAML: %v", err)
	}

	// Update indexes for all entries
	for key, entry := range tempData {
		if entry.TTL > 0 && time.Now().Unix() > entry.Timestamp+entry.TTL {
			// Skip expired entries
			continue
		}

		if err := s.indexes.Update(key, entry.Value); err != nil {
			return fmt.Errorf("failed to update indexes for key %s: %v", key, err)
		}
	}

	// Only update the main data map after all processing is successful
	s.data = tempData

	// Update statistics
	s.updateStats(int64(size))

	return nil
}

// findContentSize efficiently finds the content size by looking for YAML document end or null terminator
func (s *Store) findContentSize() int {
	// Look for null terminator first (faster)
	for i := 0; i < len(s.mm); i++ {
		if s.mm[i] == 0 {
			return i
		}
	}
	return len(s.mm)
}

func (s *Store) updateReadStats(duration time.Duration) {
	s.stats.Lock()
	defer s.stats.Unlock()

	currentAvg := s.stats.PerformanceStats.AvgReadLatency
	newLatency := duration.Seconds() * 1000 // Convert to milliseconds
	alpha := 0.1                            // Smoothing factor

	s.stats.PerformanceStats.AvgReadLatency = (alpha * newLatency) + ((1 - alpha) * currentAvg)
	s.stats.Reads++
}

func (s *Store) updateWriteStats(duration time.Duration) {
	s.stats.Lock()
	defer s.stats.Unlock()

	currentAvg := s.stats.PerformanceStats.AvgWriteLatency
	newLatency := duration.Seconds() * 1000 // Convert to milliseconds
	alpha := 0.1                            // Smoothing factor

	s.stats.PerformanceStats.AvgWriteLatency = (alpha * newLatency) + ((1 - alpha) * currentAvg)
	s.stats.Writes++
	s.stats.EntryCount = uint64(len(s.data))
}

func (s *Store) updateSyncStats(duration time.Duration) {
	s.stats.Lock()
	defer s.stats.Unlock()

	currentAvg := s.stats.PerformanceStats.AvgSyncLatency
	newLatency := duration.Seconds() * 1000 // Convert to milliseconds
	alpha := 0.1                            // Smoothing factor

	s.stats.PerformanceStats.AvgSyncLatency = (alpha * newLatency) + ((1 - alpha) * currentAvg)
	s.stats.LastSyncTime = time.Now()
	s.stats.SyncCount++
	s.stats.FileSize = int64(len(s.mm))
}

func (s *Store) updateStats(dataSize int64) {
	s.stats.Lock()
	defer s.stats.Unlock()

	s.stats.DataSize = dataSize
	s.stats.EntryCount = uint64(len(s.data))
	s.stats.FileSize = int64(len(s.mm))
}

// gcExpiredEntries removes expired entries and updates statistics
func (s *Store) gcExpiredEntries() {
	s.Lock()
	defer s.Unlock()

	now := time.Now().Unix()
	expiredCount := uint64(0)

	for key, entry := range s.data {
		if entry.TTL > 0 && now > entry.Timestamp+entry.TTL {
			delete(s.data, key)
			s.indexes.Remove(key)
			expiredCount++
			s.dirty = true
		}
	}

	if expiredCount > 0 {
		s.stats.Lock()
		s.stats.ExpiredCount += expiredCount
		s.stats.EntryCount = uint64(len(s.data))
		s.stats.PerformanceStats.LastGC = time.Now()
		s.stats.Unlock()
	}
}

// GetStats returns enhanced statistics
func (s *Store) GetStats() StoreStats {
	s.stats.Lock()
	defer s.stats.Unlock()

	// Update current stats
	s.stats.EntryCount = uint64(len(s.data))
	s.stats.FileSize = int64(len(s.mm))

	// Update index stats
	s.stats.IndexStats.TextIndexes.Count = len(s.indexes.text)
	s.stats.IndexStats.VectorIndexes.Count = len(s.indexes.vectors)
	s.stats.IndexStats.BTreeIndexes.Count = len(s.indexes.trees)

	// Count entries in indexes
	for _, idx := range s.indexes.text {
		s.stats.IndexStats.TextIndexes.EntryCount += len(idx.docs)
	}
	for _, idx := range s.indexes.vectors {
		s.stats.IndexStats.VectorIndexes.EntryCount += len(idx.vectors)
	}
	for _, tree := range s.indexes.trees {
		s.stats.IndexStats.BTreeIndexes.EntryCount += tree.Len()
	}

	return s.stats
}
