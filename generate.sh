#!/bin/bash

# Generate Go code from protobuf definitions

set -e

echo "Generating protobuf code..."

# Install protoc-gen-go if not present
if ! command -v protoc-gen-go &> /dev/null; then
    echo "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

# Generate code for test.proto
protoc --go_out=. --go_opt=paths=source_relative \
    pbmodel/testdata/test.proto

echo "Code generation complete!"
