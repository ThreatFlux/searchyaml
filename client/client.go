package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client represents a SearchYAML client
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new SearchYAML client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: time.Second * 10,
		},
	}
}

// Set stores a value with an optional TTL
func (c *Client) Set(key string, value interface{}, ttl time.Duration) error {
	url := fmt.Sprintf("%s/data/%s", c.baseURL, key)
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if ttl > 0 {
		req.Header.Set("X-TTL", ttl.String())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// Get retrieves a value by key
func (c *Client) Get(key string) (interface{}, error) {
	url := fmt.Sprintf("%s/data/%s", c.baseURL, key)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result[key], nil
}

// Delete removes a value by key
func (c *Client) Delete(key string) error {
	url := fmt.Sprintf("%s/data/%s", c.baseURL, key)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// TextSearch performs a text-based search
func (c *Client) TextSearch(text string, maxResults int, minScore float64) ([]SearchResult, error) {
	url := fmt.Sprintf("%s/search/text", c.baseURL)
	body := map[string]interface{}{
		"text":        text,
		"max_results": maxResults,
		"min_score":   minScore,
	}

	return c.search(url, body)
}

// VectorSearch performs a vector-based search
func (c *Client) VectorSearch(vector []float32, maxResults int, minScore float64) ([]SearchResult, error) {
	url := fmt.Sprintf("%s/search/vector", c.baseURL)
	body := map[string]interface{}{
		"vector":      vector,
		"max_results": maxResults,
		"min_score":   minScore,
	}

	return c.search(url, body)
}

// CombinedSearch performs a combined search with multiple criteria
func (c *Client) CombinedSearch(query SearchQuery) ([]SearchResult, error) {
	url := fmt.Sprintf("%s/search/combined", c.baseURL)
	return c.search(url, query)
}

// Helper function for search requests
func (c *Client) search(url string, body interface{}) ([]SearchResult, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var results []SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	return results, nil
}

// Types copied from storage package for client use

type SearchQuery struct {
	Text       string                 `json:"text,omitempty"`
	Vector     []float32              `json:"vector,omitempty"`
	Filters    map[string]interface{} `json:"filters,omitempty"`
	MaxResults int                    `json:"max_results,omitempty"`
	MinScore   float64                `json:"min_score,omitempty"`
}

type SearchResult struct {
	Key       string      `json:"key"`
	Value     interface{} `json:"value"`
	TextScore float64     `json:"text_score,omitempty"`
	VecScore  float32     `json:"vector_score,omitempty"`
	Combined  float64     `json:"combined_score"`
}
