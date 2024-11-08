# SearchYAML

SearchYAML is a high-performance, memory-mapped key-value store with built-in search capabilities, designed to bridge the gap between traditional databases and modern AI/ML workloads. It combines efficient CRUD operations with native AI capabilities, using YAML as its primary data format.

## Features

### Core Capabilities
- Memory-mapped YAML storage
- High-performance CRUD operations
- Built-in text and vector search
- TTL support for entries
- Automatic file size management
- Concurrent access support

### Search Features
- Text search with trigram indexing
- Vector similarity search
- Hybrid search combining text and vector results
- Configurable search parameters
- Real-time index updates

### Performance
- Write: ~900k ops/sec
- Read: ~1.2M ops/sec
- Average Latency: 1-5ms
- Memory-efficient design
- Optimized YAML encoding/decoding

## Quick Start

### Installation
```bash
go get github.com/threatflux/searchYAML
```

### Basic Usage
```go
import "github.com/threatflux/searchYAML/storage"

// Create a new store
opts := storage.DefaultOptions
store, err := storage.NewStore("data.yaml", opts)
if err != nil {
    log.Fatal(err)
}
defer store.Close()

// Set a value
err = store.Set("key", map[string]interface{}{
    "title": "Example",
    "description": "Sample document",
    "embedding": []float32{0.1, 0.2, 0.3},
})

// Get a value
entry, exists := store.Get("key")

// Search
results, err := store.Search(storage.SearchQuery{
    Text: "example",
    MaxResults: 10,
    MinScore: 0.5,
})
```

### Running the Server
```bash
go run main.go --port=:8080 --data=data.yaml
```

## API Endpoints

### CRUD Operations
- `GET /data/:key` - Retrieve a value
- `POST /data/:key` - Store a value
- `DELETE /data/:key` - Delete a value

### Search Operations
- `POST /search/text` - Text-based search
- `POST /search/vector` - Vector similarity search
- `POST /search/combined` - Combined text and vector search

### Index Management
- `POST /index/create` - Create a new index
- `DELETE /index/remove` - Remove an existing index

### Administrative
- `POST /admin/sync` - Force sync to disk
- `GET /admin/stats` - Get store statistics

## Configuration

### Store Options
```go
type StoreOptions struct {
    InitialSize  int64         // Initial file size
    MaxSize      int64         // Maximum file size
    SyncInterval time.Duration // Sync interval
    Debug        bool          // Enable debug logging
}
```

### Default Values
```go
var DefaultOptions = StoreOptions{
    InitialSize:  32 << 20,    // 32MB
    MaxSize:      512 << 20,   // 512MB
    SyncInterval: time.Minute,
    Debug:        false,
}
```

## Performance Statistics

The store maintains detailed statistics accessible via the `/admin/stats` endpoint:

- Operation counts (reads, writes, deletes)
- Latency metrics (read, write, sync)
- Storage utilization
- Index statistics
- Garbage collection metrics

## Index Types

### Text Index
- Trigram-based indexing
- Fuzzy search support
- Field-specific searches

### Vector Index
- Support for multiple embeddings per document
- Cosine similarity search
- Configurable dimensions

### BTree Index
- Ordered index for scalar values
- Range query support
- Efficient updates

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Performance Comparison

Recent benchmarks comparing SearchYAML with PostgreSQL:

```
SearchYAML vs PostgreSQL (p99 latencies):
CREATE: 1.97ms vs 2.08ms
READ: 0.80ms vs 1.35ms
```

## Roadmap

- [ ] Advanced vector quantization
- [ ] Cluster support
- [ ] Enhanced RAG capabilities
- [ ] Management UI
- [ ] Cloud integration
- [ ] Additional client libraries
