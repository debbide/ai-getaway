#!/bin/sh

# Ensure logs directory exists for the Go backend
mkdir -p /app/logs

# Start the Go backend in the background
echo "Starting ai-getaway backend..."
/app/ai-getaway &

# Wait a little bit for the backend to start
sleep 2

# Start Nginx in the foreground
echo "Starting Nginx frontend server..."
nginx -g 'daemon off;'
