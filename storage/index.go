package storage

import (
	"fmt"
	"github.com/google/btree"
	"sync"
)

// IndexManager handles multiple index types
type IndexManager struct {
	sync.RWMutex
	trees   map[string]*btree.BTree  // Field-based btree indexes
	vectors map[string]*VectorIndex  // Vector indexes
	text    map[string]*TrigramIndex // Text search indexes
}

// indexItem represents a single indexed value
type indexItem struct {
	key   string
	value interface{}
}

// Less implements btree.Item interface
func (i indexItem) Less(than btree.Item) bool {
	switch v := i.value.(type) {
	case string:
		return v < than.(indexItem).value.(string)
	case int:
		return v < than.(indexItem).value.(int)
	case float64:
		return v < than.(indexItem).value.(float64)
	default:
		return false
	}
}

// NewIndexManager creates a new index manager
func NewIndexManager() *IndexManager {
	return &IndexManager{
		trees:   make(map[string]*btree.BTree),
		vectors: make(map[string]*VectorIndex),
		text:    make(map[string]*TrigramIndex),
	}
}

// AddIndex creates a new index for the specified field
func (im *IndexManager) AddIndex(field string, indexType string) error {
	im.Lock()
	defer im.Unlock()

	switch indexType {
	case "btree":
		if _, exists := im.trees[field]; !exists {
			im.trees[field] = btree.New(32)
		}
	case "vector":
		if _, exists := im.vectors[field]; !exists {
			im.vectors[field] = NewVectorIndex(384) // Default to 384 dimensions
		}
	case "text":
		if _, exists := im.text[field]; !exists {
			im.text[field] = NewTrigramIndex()
		}
	default:
		return fmt.Errorf("unknown index type: %s", indexType)
	}

	return nil
}

// Update updates all indexes for a given key-value pair
func (im *IndexManager) Update(key string, value interface{}) error {
	im.Lock()
	defer im.Unlock()

	// Handle map values
	if m, ok := value.(map[string]interface{}); ok {
		for field, tree := range im.trees {
			if fieldValue, exists := m[field]; exists {
				tree.ReplaceOrInsert(indexItem{key, fieldValue})
			}
		}

		for field, vec := range im.vectors {
			if fieldValue, exists := m[field]; exists {
				if vectors, ok := fieldValue.([]float32); ok {
					vec.Update(key, vectors)
				}
			}
		}

		for field, idx := range im.text {
			if fieldValue, exists := m[field]; exists {
				if text, ok := fieldValue.(string); ok {
					idx.Update(key, text)
				}
			}
		}
	}

	return nil
}

// Remove removes a key from all indexes
func (im *IndexManager) Remove(key string) {
	im.Lock()
	defer im.Unlock()

	// Remove from btree indexes
	for _, tree := range im.trees {
		tree.Delete(btree.Item(indexItem{key, nil}))

	}

	// Remove from vector indexes
	for _, vec := range im.vectors {
		vec.Remove(key)
	}

	// Remove from text indexes
	for _, idx := range im.text {
		idx.Remove(key)
	}
}

// RemoveIndex removes an index of the specified type
func (im *IndexManager) RemoveIndex(field string, indexType string) error {
	im.Lock()
	defer im.Unlock()

	switch indexType {
	case "btree":
		if _, exists := im.trees[field]; exists {
			delete(im.trees, field)
			return nil
		}
	case "vector":
		if _, exists := im.vectors[field]; exists {
			delete(im.vectors, field)
			return nil
		}
	case "text":
		if _, exists := im.text[field]; exists {
			delete(im.text, field)
			return nil
		}
	default:
		return fmt.Errorf("unknown index type: %s", indexType)
	}

	return fmt.Errorf("index not found: %s (%s)", field, indexType)
}

// Search performs a search across all relevant indexes
func (im *IndexManager) Search(query map[string]interface{}) ([]string, error) {
	im.RLock()
	defer im.RUnlock()

	results := make(map[string]struct{})
	first := true

	for field, value := range query {
		if tree, exists := im.trees[field]; exists {
			fieldResults := make(map[string]struct{})
			tree.AscendGreaterOrEqual(indexItem{"", value}, func(i btree.Item) bool {
				item := i.(indexItem)
				if item.value == value {
					fieldResults[item.key] = struct{}{}
				}
				return true
			})

			if first {
				results = fieldResults
				first = false
			} else {
				// Intersect results
				for k := range results {
					if _, exists := fieldResults[k]; !exists {
						delete(results, k)
					}
				}
			}
		}
	}

	// Convert results to slice
	keys := make([]string, 0, len(results))
	for k := range results {
		keys = append(keys, k)
	}

	return keys, nil
}
