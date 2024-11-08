package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type TestData struct {
	Content struct {
		Name  string `yaml:"name" json:"name"`
		Value int    `yaml:"value" json:"value"`
	} `yaml:"content" json:"content"`
	Metadata struct {
		Created time.Time `yaml:"created" json:"created"`
		Tags    []string  `yaml:"tags" json:"tags"`
	} `yaml:"metadata" json:"metadata"`
}

type TestResult struct {
	Duration  float64
	Operation string
	Error     error
}

type Stats struct {
	Min     float64 `yaml:"min"`
	Max     float64 `yaml:"max"`
	Mean    float64 `yaml:"mean"`
	Median  float64 `yaml:"median"`
	P95     float64 `yaml:"p95"`
	P99     float64 `yaml:"p99"`
	StdDev  float64 `yaml:"stddev"`
	Samples int     `yaml:"samples"`
}

type Config struct {
	NumOperations    int      `yaml:"num_operations"`
	Concurrency      int      `yaml:"concurrency"`
	WarmupIterations int      `yaml:"warmup_iterations"`
	CooldownSeconds  int      `yaml:"cooldown_seconds"`
	BaseURL          string   `yaml:"base_url"`
	TestData         TestData `yaml:"test_data"`
}

func calculateStats(times []float64) Stats {
	if len(times) == 0 {
		return Stats{}
	}

	sorted := make([]float64, len(times))
	copy(sorted, times)
	sort.Float64s(sorted)

	sum := 0.0
	for _, t := range times {
		sum += t
	}
	mean := sum / float64(len(times))

	sumSquares := 0.0
	for _, t := range times {
		sumSquares += math.Pow(t-mean, 2)
	}
	stdDev := math.Sqrt(sumSquares / float64(len(times)))

	p95Index := int(float64(len(sorted)) * 0.95)
	p99Index := int(float64(len(sorted)) * 0.99)

	return Stats{
		Min:     sorted[0],
		Max:     sorted[len(sorted)-1],
		Mean:    mean,
		Median:  sorted[len(sorted)/2],
		P95:     sorted[p95Index],
		P99:     sorted[p99Index],
		StdDev:  stdDev,
		Samples: len(times),
	}
}

func warmup(config Config, client *http.Client) {
	log.Println("Performing warmup operations...")

	for i := 0; i < config.WarmupIterations; i++ {
		// Create test data
		result := testSearchYAMLCreate(client, config, i)
		if result.Error != nil {
			log.Printf("Warmup create error: %v", result.Error)
			continue
		}

		// Wait a bit for data to be indexed
		time.Sleep(100 * time.Millisecond)

		// Test read
		result = testSearchYAMLRead(client, config, i)
		if result.Error != nil {
			log.Printf("Warmup read error: %v", result.Error)
		}

		// Test search
		result = testSearchYAMLSearch(client, config, i)
		if result.Error != nil {
			log.Printf("Warmup search error: %v", result.Error)
		}

		time.Sleep(time.Second)
	}

	log.Println("Warmup complete")
}

func testSearchYAMLCreate(client *http.Client, config Config, i int) TestResult {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)

	testData := config.TestData
	testData.Metadata.Created = time.Now()
	testData.Metadata.Tags = []string{"test", fmt.Sprintf("iteration_%d", i)}

	if err := encoder.Encode(testData); err != nil {
		return TestResult{Operation: "create", Error: fmt.Errorf("encode error: %v", err)}
	}
	encoder.Close()

	// Update URL to match the API structure
	url := fmt.Sprintf("%s/data/test_key_%d", config.BaseURL, i)
	start := time.Now()

	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return TestResult{Operation: "create", Error: fmt.Errorf("request creation error: %v", err)}
	}

	req.Header.Set("Content-Type", "application/x-yaml")
	req.Header.Set("Accept", "application/x-yaml")

	resp, err := client.Do(req)
	if err != nil {
		return TestResult{Operation: "create", Error: fmt.Errorf("request error: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return TestResult{Operation: "create", Error: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
	}

	duration := time.Since(start).Seconds() * 1000
	return TestResult{Duration: duration, Operation: "create"}
}
func testSearchYAMLRead(client *http.Client, config Config, i int) TestResult {
	url := fmt.Sprintf("%s/data/test_key_%d", config.BaseURL, i)
	start := time.Now()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return TestResult{Operation: "read", Error: fmt.Errorf("request creation error: %v", err)}
	}

	req.Header.Set("Accept", "application/x-yaml")

	resp, err := client.Do(req)
	if err != nil {
		return TestResult{Operation: "read", Error: fmt.Errorf("request error: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return TestResult{Operation: "read", Error: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
	}

	var result map[string]TestData
	decoder := yaml.NewDecoder(resp.Body)
	if err := decoder.Decode(&result); err != nil {
		return TestResult{Operation: "read", Error: fmt.Errorf("decode error: %v", err)}
	}

	duration := time.Since(start).Seconds() * 1000
	return TestResult{Duration: duration, Operation: "read"}
}

func testSearchYAMLSearch(client *http.Client, config Config, i int) TestResult {
	searchQuery := struct {
		Text       string  `json:"text"`
		MaxResults int     `json:"max_results"`
		MinScore   float64 `json:"min_score"`
	}{
		Text:       fmt.Sprintf("test iteration_%d", i),
		MaxResults: 10,
		MinScore:   0.5,
	}

	// Use json.Marshal instead of YAML encoder for search endpoint
	jsonData, err := json.Marshal(searchQuery)
	if err != nil {
		return TestResult{Operation: "search", Error: fmt.Errorf("json encode error: %v", err)}
	}

	url := fmt.Sprintf("%s/search/text", config.BaseURL)
	start := time.Now()

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return TestResult{Operation: "search", Error: fmt.Errorf("request creation error: %v", err)}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return TestResult{Operation: "search", Error: fmt.Errorf("request error: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return TestResult{Operation: "search", Error: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
	}

	duration := time.Since(start).Seconds() * 1000
	return TestResult{Duration: duration, Operation: "search"}
}

func runTests(config Config, testType string) (map[string][]float64, error) {
	results := make(map[string][]float64)
	results["create"] = make([]float64, 0, config.NumOperations)
	results["read"] = make([]float64, 0, config.NumOperations)
	results["search"] = make([]float64, 0, config.NumOperations)

	transport := &http.Transport{
		MaxIdleConns:        config.Concurrency,
		MaxIdleConnsPerHost: config.Concurrency,
		IdleConnTimeout:     30 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	warmup(config, client)

	var wg sync.WaitGroup
	resultChan := make(chan TestResult, config.NumOperations*3)
	rateLimiter := make(chan struct{}, config.Concurrency)

	testFuncs := map[string]func(*http.Client, Config, int) TestResult{
		"create": testSearchYAMLCreate,
		"read":   testSearchYAMLRead,
		"search": testSearchYAMLSearch,
	}

	for op, testFunc := range testFuncs {
		if testType == "" || testType == op {
			log.Printf("Running %s tests...", op)
			for i := 0; i < config.NumOperations; i++ {
				wg.Add(1)
				go func(index int, operation string, tf func(*http.Client, Config, int) TestResult) {
					defer wg.Done()
					rateLimiter <- struct{}{}
					result := tf(client, config, index)
					<-rateLimiter
					resultChan <- result
				}(i, op, testFunc)
			}
			time.Sleep(time.Duration(config.CooldownSeconds) * time.Second)
		}
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var errors []error
	errorCount := make(map[string]int)

	for result := range resultChan {
		if result.Error != nil {
			errors = append(errors, fmt.Errorf("%s error: %v", result.Operation, result.Error))
			errorCount[result.Operation]++
			continue
		}
		results[result.Operation] = append(results[result.Operation], result.Duration)
	}

	if len(errors) > 0 {
		// Log the first few errors for each operation type
		for op, count := range errorCount {
			log.Printf("Operation %s had %d errors", op, count)
			if len(errors) > 0 {
				log.Printf("Sample errors for %s:", op)
				for i := 0; i < min(3, len(errors)); i++ {
					if strings.HasPrefix(errors[i].Error(), op) {
						log.Printf("  %v", errors[i])
					}
				}
			}
		}
		return results, fmt.Errorf("encountered errors during testing: %d total errors", len(errors))
	}

	return results, nil
}

func main() {
	numOps := flag.Int("n", 10000, "number of operations")
	concurrency := flag.Int("c", 10, "concurrency level")
	warmupIters := flag.Int("w", 3, "number of warmup iterations")
	cooldown := flag.Int("cooldown", 2, "cooldown time between tests in seconds")
	baseURL := flag.String("url", "http://localhost:8080", "base URL for SearchYAML")
	testType := flag.String("type", "", "test type (create/read/search/empty for all)")
	flag.Parse()

	config := Config{
		NumOperations:    *numOps,
		Concurrency:      *concurrency,
		WarmupIterations: *warmupIters,
		CooldownSeconds:  *cooldown,
		BaseURL:          *baseURL,
		TestData: TestData{
			Content: struct {
				Name  string `yaml:"name" json:"name"`
				Value int    `yaml:"value" json:"value"`
			}{
				Name:  "#{ZrkwS*2)7osH{RCJz.UPZvN>}L%)$DFyZ,K:Vs1lTKc0}QKzC3eG&[dgFxFZO+XrZ{x^.]#HQ&Odv,YRaqtJvr!szmVd8p,y5+|$vz{3O<48HVgyxV$}NHZhzxy^>^\n",
				Value: 1234567890,
			},
			Metadata: struct {
				Created time.Time `yaml:"created" json:"created"`
				Tags    []string  `yaml:"tags" json:"tags"`
			}{
				Created: time.Now(),
				Tags:    []string{"#{ZrkwS*2)7osH{RCJz.UPZvN>}L%)$DFyZ,K:Vs1lTKc0}QKzC3eG&[dgFxFZO+XrZ{x^.]#HQ&Odv,YRaqtJvr!szmVd8p,y5+|$vz{3O<48HVgyxV$}NHZhzxy^>^\n"},
			},
		},
	}

	log.Printf("Starting YAML performance tests with configuration:")
	log.Printf("  Operations: %d", config.NumOperations)
	log.Printf("  Concurrency: %d", config.Concurrency)
	log.Printf("  URL: %s", config.BaseURL)

	results, err := runTests(config, *testType)
	if err != nil {
		log.Fatalf("Error running tests: %v", err)
	}

	fmt.Println("\nYAML Performance Test Results")
	fmt.Println("============================")

	for op, times := range results {
		if len(times) > 0 {
			stats := calculateStats(times)
			printStats(stats, op)
		}
	}

	if err := saveResults(results, config); err != nil {
		log.Printf("Error saving results: %v", err)
	}
}

func saveResults(results map[string][]float64, config Config) error {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("searchyaml_test_results_%s.yaml", timestamp)

	stats := make(map[string]Stats)
	for op, times := range results {
		stats[op] = calculateStats(times)
	}

	output := struct {
		Config  Config               `yaml:"config"`
		Stats   map[string]Stats     `yaml:"stats"`
		Results map[string][]float64 `yaml:"raw_results"`
	}{
		Config:  config,
		Stats:   stats,
		Results: results,
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	return encoder.Encode(output)
}

func printStats(stats Stats, operation string) {
	fmt.Printf("\n%s Operations:\n", operation)
	fmt.Printf("  Min: %.2fms\n", stats.Min)
	fmt.Printf("  Max: %.2fms\n", stats.Max)
	fmt.Printf("  Mean: %.2fms\n", stats.Mean)
	fmt.Printf("  Median: %.2fms\n", stats.Median)
	fmt.Printf("  P95: %.2fms\n", stats.P95)
	fmt.Printf("  P99: %.2fms\n", stats.P99)
	fmt.Printf("  StdDev: %.2fms\n", stats.StdDev)
	fmt.Printf("  Samples: %d\n", stats.Samples)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
