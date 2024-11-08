package storage

import (
	"sort"
	"strings"
	"sync"
)

// TrigramIndex provides text search using trigram-based indexing
type TrigramIndex struct {
	sync.RWMutex
	trigrams map[string]map[string]struct{} // trigram -> document keys
	docs     map[string]string              // document key -> original text
}

// TextSearchResult represents a single text search result with score
type TextSearchResult struct {
	Key   string
	Score float64
	Text  string
}

// NewTrigramIndex creates a new trigram-based text index
func NewTrigramIndex() *TrigramIndex {
	return &TrigramIndex{
		trigrams: make(map[string]map[string]struct{}),
		docs:     make(map[string]string),
	}
}

// Update adds or updates a document in the index
func (ti *TrigramIndex) Update(key string, text string) {
	ti.Lock()
	defer ti.Unlock()

	// Remove old trigrams if document exists
	if oldText, exists := ti.docs[key]; exists {
		ti.removeDocumentTrigrams(key, oldText)
	}

	// Store original text
	ti.docs[key] = text

	// Generate and store trigrams
	for _, trigram := range generateTrigrams(text) {
		if ti.trigrams[trigram] == nil {
			ti.trigrams[trigram] = make(map[string]struct{})
		}
		ti.trigrams[trigram][key] = struct{}{}
	}
}

// Remove deletes a document from the index
func (ti *TrigramIndex) Remove(key string) {
	ti.Lock()
	defer ti.Unlock()

	if text, exists := ti.docs[key]; exists {
		ti.removeDocumentTrigrams(key, text)
		delete(ti.docs, key)
	}
}

// Search performs a fuzzy text search using trigrams
func (ti *TrigramIndex) Search(query string, maxResults int) []TextSearchResult {
	ti.RLock()
	defer ti.RUnlock()

	// Generate query trigrams
	queryTrigrams := generateTrigrams(query)

	// Count trigram matches per document
	scores := make(map[string]int)
	for _, trigram := range queryTrigrams {
		if docs, exists := ti.trigrams[trigram]; exists {
			for doc := range docs {
				scores[doc]++
			}
		}
	}

	// Convert to results slice and calculate normalized scores
	results := make([]TextSearchResult, 0, len(scores))
	maxQueryTrigrams := len(queryTrigrams)
	for doc, matches := range scores {
		score := float64(matches) / float64(maxQueryTrigrams)
		results = append(results, TextSearchResult{
			Key:   doc,
			Score: score,
			Text:  ti.docs[doc],
		})
	}

	// Sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit results
	if maxResults > 0 && len(results) > maxResults {
		results = results[:maxResults]
	}

	return results
}

// Helper functions

func (ti *TrigramIndex) removeDocumentTrigrams(key string, text string) {
	for _, trigram := range generateTrigrams(text) {
		if docs, exists := ti.trigrams[trigram]; exists {
			delete(docs, key)
			if len(docs) == 0 {
				delete(ti.trigrams, trigram)
			}
		}
	}
}

func generateTrigrams(text string) []string {
	text = strings.ToLower(text)
	if len(text) < 3 {
		return []string{text}
	}

	trigrams := make([]string, 0, len(text)-2)
	for i := 0; i <= len(text)-3; i++ {
		trigrams = append(trigrams, text[i:i+3])
	}
	return trigrams
}

// FuzzySearch performs fuzzy text search with configurable parameters
func (ti *TrigramIndex) FuzzySearch(query string, minScore float64, maxResults int) []TextSearchResult {
	results := ti.Search(query, 0) // Get all results first

	// Filter by minimum score
	filtered := make([]TextSearchResult, 0, len(results))
	for _, result := range results {
		if result.Score >= minScore {
			filtered = append(filtered, result)
		}
	}

	// Limit results
	if maxResults > 0 && len(filtered) > maxResults {
		filtered = filtered[:maxResults]
	}

	return filtered
}
