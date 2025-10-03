#!/bin/bash
# Run Crush-RCM with error capture but normal TUI

echo "🔧 Starting Crush-RCM..."
echo "📊 Database: rcm_system"
echo "📁 Error log: /tmp/crush-errors.log"
echo ""
echo "If an error occurs, check: cat /tmp/crush-errors.log"
echo ""
sleep 2

# Set PostgreSQL connection to rcm_system database
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=token_saver
export POSTGRES_PASSWORD=token_saver_secure_pwd_2024
export POSTGRES_DB=rcm_system

# Clear previous error log
> /tmp/crush-errors.log

# Run normally but redirect stderr to file
cd /Users/jerry/VSCode/crush-rcm
./crush-rcm --debug 2>/tmp/crush-errors.log