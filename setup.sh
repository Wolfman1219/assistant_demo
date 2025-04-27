#!/bin/bash

# Exit on error
set -e

# Print commands
set -x

# Create necessary directories
mkdir -p static proto/vad proto/trigger proto/stt proto/tts

# Copy frontend files to static directory
cp index.html static/
cp style.css static/
cp app.js static/
cp audio-processor.js static/

# Install required Go packages
go mod tidy

# Install protoc if not already installed
if ! command -v protoc &> /dev/null; then
    echo "Installing protoc..."
    # For Linux
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        PROTOC_ZIP=protoc-3.19.4-linux-x86_64.zip
        curl -OL https://github.com/protocolbuffers/protobuf/releases/download/v3.19.4/$PROTOC_ZIP
        sudo unzip -o $PROTOC_ZIP -d /usr/local bin/protoc
        sudo unzip -o $PROTOC_ZIP -d /usr/local 'include/*'
        rm -f $PROTOC_ZIP
    # For macOS
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        brew install protobuf
    else
        echo "Please install protoc manually for your platform: https://grpc.io/docs/protoc-installation/"
        exit 1
    fi
fi

# Install protoc-gen-go and protoc-gen-go-grpc if not already installed
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

# Generate Go code from proto files
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/vad/vad.proto proto/trigger/trigger.proto proto/stt/stt.proto proto/tts/tts.proto

echo "Setup completed successfully!"
echo "Run 'make run' to start the application."