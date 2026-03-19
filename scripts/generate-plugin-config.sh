#!/bin/bash
# Copy static plugin configuration files to release directory.
# Called from .goreleaser.yaml before hooks.

set -e

# Source and destination directories
SOURCE_DIR="plugin/engram/.claude-plugin"
OUTPUT_DIR=".claude-plugin"

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Copy static files (no template substitution needed)
cp "$SOURCE_DIR/plugin.json" "$OUTPUT_DIR/plugin.json"
echo "Copied $OUTPUT_DIR/plugin.json"

cp "$SOURCE_DIR/marketplace.json" "$OUTPUT_DIR/marketplace.json"
echo "Copied $OUTPUT_DIR/marketplace.json"

echo "Plugin config files copied successfully"
