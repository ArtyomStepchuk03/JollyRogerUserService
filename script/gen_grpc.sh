#!/bin/bash

PROTO_DIR=proto
OUT_DIR=pb

cd "$(dirname "$0")/.."

echo "📦 Generating gRPC code from .proto..."

protoc \
  --proto_path=proto \
  --go_out=$OUT_DIR \
  --go-grpc_out=$OUT_DIR \
  --go_opt=paths=source_relative \
  --go-grpc_opt=paths=source_relative \
  $PROTO_DIR/user.proto

echo "✅ Done!"
