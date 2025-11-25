#!/bin/bash

echo "🚀 Setting up Distributed Task Queue System"
echo "============================================"
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21 or higher."
    exit 1
fi

echo "✅ Go version: $(go version)"
echo ""

# Download dependencies
echo "📦 Downloading Go dependencies..."
go mod download 2>&1
if [ $? -ne 0 ]; then
    echo "⚠️  go mod download failed, trying go mod tidy..."
    go mod tidy 2>&1
fi
echo ""

# Build the application
echo "🔨 Building the application..."
go build -o task-queue cmd/server/main.go
if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
else
    echo "❌ Build failed. Please check for errors above."
    exit 1
fi
echo ""

echo "✅ Setup complete!"
echo ""
echo "To start the server, run:"
echo "  ./task-queue"
echo ""
echo "Or run directly with:"
echo "  go run cmd/server/main.go"
echo ""
echo "Then open http://localhost:8080 in your browser"
