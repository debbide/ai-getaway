# Stage 1: Build Frontend (Vue 3)
FROM node:18-alpine AS frontend-builder
WORKDIR /app
# Copy package files
COPY frontend/package*.json ./
# Install dependencies
RUN npm install
# Copy frontend source
COPY frontend/ ./
# Build production files
RUN npm run build

# Stage 2: Build Backend (Go)
FROM golang:1.21-alpine AS backend-builder
WORKDIR /app
# Install build dependencies
RUN apk add --no-cache gcc musl-dev
# Copy go mod files
COPY go.mod go.sum* ./
# Download dependencies
RUN go mod download
# Copy backend source
COPY . .
# Build the binary
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ai-getaway .

# Stage 3: Final Alpine Image
FROM alpine:latest
WORKDIR /app

# Install Nginx and other necessary tools
RUN apk add --no-cache nginx tzdata ca-certificates

# Copy Nginx configuration
COPY nginx.conf /etc/nginx/nginx.conf

# Copy frontend build from stage 1 to Nginx default html directory
COPY --from=frontend-builder /app/dist /usr/share/nginx/html

# Copy backend binary from stage 2
COPY --from=backend-builder /app/ai-getaway /app/ai-getaway

# Copy startup script
COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh

# Expose Nginx port
EXPOSE 80

# Environment variables
ENV APP_ENV=production
ENV APP_PORT=8080 

# Start script
CMD ["/app/start.sh"]
