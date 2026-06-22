#!/bin/sh

# Ensure logs directory exists for the Go backend
mkdir -p /app/logs

# Start Nginx in the background
echo "Starting Nginx frontend server..."
nginx

# Start the Go backend in the foreground (taking over PID 1)
# If the backend crashes (e.g. database not ready), the container will exit
# and Docker's "restart: always" will automatically restart it!
echo "Starting ai-getaway backend..."
exec /app/ai-getaway
