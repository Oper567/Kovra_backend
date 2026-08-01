#!/bin/sh

echo "Starting Kovra Microservices (Free-Tier Monolith Mode)..."

# Render provides the public port in $PORT. We need the API Gateway to bind to it.
export GATEWAY_PORT=${PORT:-8080}

# Default downstream URLs for the API Gateway to route to localhost
export WALLET_SERVICE_URL="http://127.0.0.1:8081"
export NOTIFICATION_SERVICE_URL="http://127.0.0.1:8087"

# 1. Start Wallet Service
echo "Starting Wallet Service on port 8081..."
SERVER_PORT=8081 GRPC_PORT=9081 ./wallet-service &

# 2. Start Notification Service
echo "Starting Notification Service on port 8087..."
SERVER_PORT=8087 ./notification-service &

# Wait a moment for services to bind
sleep 2

# 3. Start API Gateway (Runs in foreground to keep container alive)
echo "Starting API Gateway on port $GATEWAY_PORT..."
./api-gateway
