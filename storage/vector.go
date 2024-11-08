package storage

import (
	"fmt"
	"math"
	"sort"
	"sync"
)

// VectorIndex provides vector similarity search capabilities
type VectorIndex struct {
	sync.RWMutex
	vectors map[string][]float32
	dim     int
}

// VectorSearchResult represents a single search result with score
type VectorSearchResult struct {
	Key   string
	Score float32
}

// NewVectorIndex creates a new vector index with specified dimensions
func NewVectorIndex(dimensions int) *VectorIndex {
	return &VectorIndex{
		vectors: make(map[string][]float32),
		dim:     dimensions,
	}
}

// Update adds or updates a vector for a given key
func (vi *VectorIndex) Update(key string, vector []float32) error {
	vi.Lock()
	defer vi.Unlock()

	if len(vector) != vi.dim {
		return fmt.Errorf("vector dimension mismatch: expected %d, got %d", vi.dim, len(vector))
	}

	// Normalize vector before storing
	normalized := make([]float32, len(vector))
	copy(normalized, vector)
	normalizeVector(normalized)

	vi.vectors[key] = normalized
	return nil
}

// Remove deletes a vector from the index
func (vi *VectorIndex) Remove(key string) {
	vi.Lock()
	defer vi.Unlock()
	delete(vi.vectors, key)
}

// Search performs approximate nearest neighbor search
func (vi *VectorIndex) Search(query []float32, k int) ([]VectorSearchResult, error) {
	vi.RLock()
	defer vi.RUnlock()

	if len(query) != vi.dim {
		return nil, fmt.Errorf("query dimension mismatch: expected %d, got %d", vi.dim, len(query))
	}

	// Normalize query vector
	normalized := make([]float32, len(query))
	copy(normalized, query)
	normalizeVector(normalized)

	// Calculate cosine similarity with all vectors
	results := make([]VectorSearchResult, 0, len(vi.vectors))
	for key, vec := range vi.vectors {
		similarity := cosineSimilarity(normalized, vec)
		results = append(results, VectorSearchResult{
			Key:   key,
			Score: similarity,
		})
	}

	// Sort by similarity score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top k results
	if k > len(results) {
		k = len(results)
	}
	return results[:k], nil
}

// Helper functions for vector operations

func normalizeVector(vec []float32) {
	var sum float32
	for _, v := range vec {
		sum += v * v
	}
	magnitude := float32(math.Sqrt(float64(sum)))
	if magnitude > 0 {
		for i := range vec {
			vec[i] /= magnitude
		}
	}
}

func cosineSimilarity(a, b []float32) float32 {
	var dotProduct float32
	for i := range a {
		dotProduct += a[i] * b[i]
	}
	return dotProduct
}

// BatchSearch performs vector search with multiple query vectors
func (vi *VectorIndex) BatchSearch(queries [][]float32, k int) ([][]VectorSearchResult, error) {
	results := make([][]VectorSearchResult, len(queries))
	var wg sync.WaitGroup
	var errCh = make(chan error, len(queries))

	for i, query := range queries {
		wg.Add(1)
		go func(idx int, q []float32) {
			defer wg.Done()
			result, err := vi.Search(q, k)
			if err != nil {
				errCh <- err
				return
			}
			results[idx] = result
		}(i, query)
	}

	wg.Wait()
	close(errCh)

	if err := <-errCh; err != nil {
		return nil, err
	}

	return results, nil
}
