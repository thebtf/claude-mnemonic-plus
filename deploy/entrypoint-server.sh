#!/bin/sh
set -e
# Worker now serves both HTTP API and MCP SSE on a single port.
exec worker
