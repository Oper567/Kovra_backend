#!/bin/sh

echo "Starting Luce Pay Microservices (Free-Tier Monolith Mode)..."

# Render provides the public port in $PORT. We need the API Gateway to bind to it.
export GATEWAY_PORT=${PORT:-8080}

# Default downstream URLs for the API Gateway to route to localhost
export WALLET_SERVICE_URL="http://127.0.0.1:8081"
export VTU_SERVICE_URL="http://127.0.0.1:8082"
export AI_SERVICE_URL="http://127.0.0.1:8085"
export NOTIFICATION_SERVICE_URL="http://127.0.0.1:8087"
export ENGAGEMENT_SERVICE_URL="http://127.0.0.1:8086"
export DATABASE_URL="postgres://postgres.kcxsqfbepqrcfmrefqlt:MHWDUdklbdFnU4Xw@aws-1-eu-west-2.pooler.supabase.com:5432/postgres?sslmode=require"

# 1. Start Wallet Service
echo "Starting Wallet Service on port 8081..."
SERVER_PORT=8081 GRPC_PORT=9081 ./wallet-service &

# 2. Start Notification Service
echo "Starting Notification Service on port 8087..."
SERVER_PORT=8087 ./notification-service &

# 3. Start VTU & Gaming Service
echo "Starting VTU & Gaming Service on port 8082..."
SERVER_PORT=8082 ./vtu-gaming-service &

# 4. Start AI Service
echo "Starting AI Service on port 8085..."
SERVER_PORT=8085 PORT=8085 ./ai-service &

# 5. Start Engagement Service
echo "Starting Engagement Service on port 8086..."
SERVER_PORT=8086 PORT=8086 ./engagement-service &

# 6. Start Marketplace Service
echo "Starting Marketplace Service on port 8083..."
SERVER_PORT=8083 PORT=8083 ./marketplace-service &

# Wait a moment for services to bind
sleep 5

# 5. Start API Gateway (Runs in foreground to keep container alive)
echo "Starting API Gateway on port $GATEWAY_PORT..."
./api-gateway
