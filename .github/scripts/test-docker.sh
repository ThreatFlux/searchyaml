#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

echo "Testing SearchYAML Docker image..."

# Function to clean up
cleanup() {
    echo "Cleaning up test containers..."
    docker rm -f test-searchyaml || true
}

# Ensure cleanup runs on script exit
trap cleanup EXIT

# Start container
echo "Starting container..."
docker run -d --name test-searchyaml \
    -p 8080:8080 \
    -e DEBUG=true \
    "$1"

# Wait for container to be healthy
echo "Waiting for container to be healthy..."
ATTEMPTS=0
MAX_ATTEMPTS=30
until [ "$(docker inspect -f {{.State.Health.Status}} test-searchyaml)" == "healthy" ] || [ $ATTEMPTS -eq $MAX_ATTEMPTS ]; do
    echo "Waiting... ($(( ATTEMPTS + 1 ))/$MAX_ATTEMPTS)"
    sleep 2
    ATTEMPTS=$(( ATTEMPTS + 1 ))
done

if [ $ATTEMPTS -eq $MAX_ATTEMPTS ]; then
    echo -e "${RED}Container failed to become healthy${NC}"
    docker logs test-searchyaml
    exit 1
fi

echo -e "${GREEN}Container is healthy${NC}"

# Test basic operations
echo "Testing basic operations..."

# Test writing data
echo "Testing write operation..."
curl -X POST -H "Content-Type: application/json" \
    -d '{"test": "data"}' \
    http://localhost:8080/data/test1

# Test reading data
echo "Testing read operation..."
RESULT=$(curl -s http://localhost:8080/data/test1)
if [[ $RESULT != *"test"* ]]; then
    echo -e "${RED}Failed to read data${NC}"
    exit 1
fi

# Test search
echo "Testing search operation..."
curl -X POST -H "Content-Type: application/json" \
    -d '{"text": "test"}' \
    http://localhost:8080/search/text

# Check stats
echo "Testing stats endpoint..."
curl -s http://localhost:8080/admin/stats

echo -e "${GREEN}All tests passed successfully!${NC}"