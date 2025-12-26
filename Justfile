# Justfile for Top Hacker News Feed

set dotenv-load

# Default recipe
default:
    @just --list

# Initialize go module
init:
    go mod init github.com/dviramontes/trmnl-tophackernews
    go mod tidy

# Run locally (requires Go installed)
run:
    go run main.go

# Run with force update (bypass cache)
run-force:
    FORCE_UPDATE=true go run main.go

# Build the binary locally
build:
    go build -o hackernews-feed main.go

# Build the Docker image
docker-build:
    docker build -t hackernews-feed .

# Run as a Docker container (one-time job)
docker-run:
    docker run --rm \
        -e GEMINI_API_KEY="${GEMINI_API_KEY}" \
        -v "$(pwd)/cache:/app/cache" \
        -v "$(pwd)/headline_images_nano_banana:/app/headline_images_nano_banana" \
        hackernews-feed

# Run Docker with force update
docker-run-force:
    docker run --rm \
        -e GEMINI_API_KEY="${GEMINI_API_KEY}" \
        -e FORCE_UPDATE=true \
        -v "$(pwd)/cache:/app/cache" \
        -v "$(pwd)/headline_images_nano_banana:/app/headline_images_nano_banana" \
        hackernews-feed

# Build and run Docker in one command
docker: docker-build docker-run

# Shell into the Docker container for debugging
docker-shell:
    docker run --rm -it \
        -e GEMINI_API_KEY="${GEMINI_API_KEY}" \
        -v "$(pwd)/cache:/app/cache" \
        -v "$(pwd)/headline_images_nano_banana:/app/headline_images_nano_banana" \
        --entrypoint /bin/sh \
        hackernews-feed

# Clean build artifacts
clean:
    rm -f hackernews-feed

# Clean up cache and images
clean-cache:
    rm -rf cache headline_images_nano_banana

# Clean Docker images
docker-clean:
    docker rmi hackernews-feed 2>/dev/null || true
