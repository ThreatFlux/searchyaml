package storage

import (
	"fmt"
	"sort"
)

// SearchQuery represents a combined search query
type SearchQuery struct {
	Text       string                 `json:"text,omitempty"`
	Vector     []float32              `json:"vector,omitempty"`
	Filters    map[string]interface{} `json:"filters,omitempty"`
	MaxResults int                    `json:"max_results,omitempty"`
	MinScore   float64                `json:"min_score,omitempty"`
}

// SearchResult represents a combined search result
type SearchResult struct {
	Key       string      `json:"key"`
	Value     interface{} `json:"value"`
	TextScore float64     `json:"text_score,omitempty"`
	VecScore  float32     `json:"vector_score,omitempty"`
	Combined  float64     `json:"combined_score"`
}

// Search performs a combined search across all indexes
func (s *Store) Search(query SearchQuery) ([]SearchResult, error) {
	s.RLock()
	defer s.RUnlock()

	var textResults []TextSearchResult
	var vectorResults []VectorSearchResult
	var filterResults []string

	// Perform text search if query contains text
	if query.Text != "" {
		for _, idx := range s.indexes.text {
			textResults = append(textResults, idx.FuzzySearch(query.Text, query.MinScore, query.MaxResults)...)
		}
	}

	// Perform vector search if query contains vector
	if len(query.Vector) > 0 {
		for _, idx := range s.indexes.vectors {
			results, err := idx.Search(query.Vector, query.MaxResults)
			if err != nil {
				return nil, fmt.Errorf("vector search error: %v", err)
			}
			vectorResults = append(vectorResults, results...)
		}
	}

	// Apply filters if present
	if len(query.Filters) > 0 {
		results, err := s.indexes.Search(query.Filters)
		if err != nil {
			return nil, fmt.Errorf("filter search error: %v", err)
		}
		filterResults = results
	}

	// Combine results
	combined := s.combineResults(textResults, vectorResults, filterResults)

	// Sort and limit results
	sortSearchResults(combined)
	if query.MaxResults > 0 && len(combined) > query.MaxResults {
		combined = combined[:query.MaxResults]
	}

	return combined, nil
}

// combineResults merges results from different search types
func (s *Store) combineResults(text []TextSearchResult, vector []VectorSearchResult, filters []string) []SearchResult {
	scores := make(map[string]*SearchResult)

	// Process text results
	for _, r := range text {
		result := &SearchResult{
			Key:       r.Key,
			TextScore: r.Score,
		}
		scores[r.Key] = result
	}

	// Process vector results
	for _, r := range vector {
		if result, exists := scores[r.Key]; exists {
			result.VecScore = r.Score
			result.Combined = calculateCombinedScore(result.TextScore, float64(r.Score))
		} else {
			scores[r.Key] = &SearchResult{
				Key:      r.Key,
				VecScore: r.Score,
				Combined: float64(r.Score),
			}
		}
	}

	// Apply filters
	if len(filters) > 0 {
		filtered := make(map[string]*SearchResult)
		for _, key := range filters {
			if result, exists := scores[key]; exists {
				filtered[key] = result
			}
		}
		scores = filtered
	}

	// Get values for results
	results := make([]SearchResult, 0, len(scores))
	for key, result := range scores {
		if entry, exists := s.data[key]; exists {
			result.Value = entry.Value
			results = append(results, *result)
		}
	}

	return results
}

// Helper functions

func calculateCombinedScore(textScore, vectorScore float64) float64 {
	// Simple weighted average - can be adjusted based on requirements
	return (textScore + vectorScore) / 2
}

func sortSearchResults(results []SearchResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Combined > results[j].Combined
	})
}
