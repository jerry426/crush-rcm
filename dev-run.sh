#!/bin/bash
# Development script to build and run Crush-RCM with debug logging

set -e  # Exit on error

echo "🔨 Building Crush-RCM..."
go build -o crush-rcm .

echo "✅ Build complete"
echo "🚀 Launching Crush-RCM with debug logging..."
echo "📊 Database: rcm_system (PostgreSQL)"
echo ""
echo "📝 To view logs in another terminal, run:"
echo "    cd /Users/jerry/VSCode/crush-rcm && ./crush-rcm logs --follow --debug"
echo ""
sleep 2

# Run with debug logging and PostgreSQL connection
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=token_saver
export POSTGRES_PASSWORD=token_saver_secure_pwd_2024
export POSTGRES_DB=rcm_system

./crush-rcm