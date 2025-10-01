#!/bin/bash
# Run Crush-RCM with full logging to file

echo "🔧 Starting Crush-RCM with full logging..."
echo "📁 Log file: /tmp/crush-full.log"
echo "📊 Database: rcm_context"
echo ""
echo "To watch the log in another terminal, run:"
echo "  tail -f /tmp/crush-full.log"
echo ""

# Set PostgreSQL connection to rcm_context database
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=token_saver
export POSTGRES_PASSWORD=token_saver_secure_pwd_2024
export POSTGRES_DB=rcm_context

# Clear previous log
> /tmp/crush-full.log

# Run with debug flag and capture ALL output
cd /Users/jerry/VSCode/crush-rcm
./crush-rcm --debug 2>&1 | tee /tmp/crush-full.log