FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files if they exist, otherwise init
COPY go.* ./
RUN go mod download 2>/dev/null || true

COPY *.go ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /hackernews-feed .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /hackernews-feed .

# Copy default image
COPY default.png .

# Create directories for cache and images
RUN mkdir -p cache headline_images_nano_banana

ENTRYPOINT ["/app/hackernews-feed"]
