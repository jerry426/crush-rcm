#!/bin/bash
# Debug run script for Crush-RCM with PostgreSQL

echo "🔧 Running Crush-RCM with debug output..."
echo "📊 Database: rcm_system (PostgreSQL)"
echo "👀 Watch for DEBUG: lines in the output"
echo ""

# Set PostgreSQL connection to rcm_system database
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=token_saver
export POSTGRES_PASSWORD=token_saver_secure_pwd_2024
export POSTGRES_DB=rcm_system

# Run with debug flag
cd /Users/jerry/VSCode/crush-rcm
./crush-rcm --debug