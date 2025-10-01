#!/bin/bash
# View Crush-RCM debug logs in real-time

echo "📝 Viewing Crush-RCM debug logs (live)..."
echo "   Press Ctrl+C to exit"
echo ""

./crush-rcm logs --follow --debug
