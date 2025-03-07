#!/bin/bash

# Script to run E2E tests and capture detailed results

# Set up error handling
set -e
trap 'echo "Error occurred. Stopping servers and exiting."; cleanup' ERR

# Function to clean up processes
cleanup() {
    echo "Cleaning up processes..."
    # Find and kill background processes by name
    pkill -f "go run main.go" || true
    pkill -f "npm run dev" || true
    exit 1
}

# Start the backend
echo "Starting Go backend..."
go run main.go &
BACKEND_PID=$!
echo "Backend started with PID: $BACKEND_PID"

# Start the frontend
echo "Starting Next.js frontend..."
cd web-ui && npm run dev &
FRONTEND_PID=$!
echo "Frontend started with PID: $FRONTEND_PID"

# Wait for services to be ready
echo "Waiting for services to be ready..."
sleep 10

# Run API tests first
echo "Running API integration tests..."
cd web-ui && npm run test:api

# Check the exit code
API_EXIT=$?
if [ $API_EXIT -eq 0 ]; then
    echo "✅ API tests PASSED"
else
    echo "❌ API tests FAILED with exit code $API_EXIT"
fi

# Run UI tests
echo "Running UI flow tests..."
cd web-ui && npm run test:ui

# Check the exit code
UI_EXIT=$?
if [ $UI_EXIT -eq 0 ]; then
    echo "✅ UI tests PASSED"
else
    echo "❌ UI tests FAILED with exit code $UI_EXIT"
fi

# Clean up
echo "Tests completed. Cleaning up processes..."
kill $BACKEND_PID $FRONTEND_PID || true

# Final report
echo "-------------------------"
echo "TEST EXECUTION SUMMARY"
echo "-------------------------"
echo "API Tests: $([ $API_EXIT -eq 0 ] && echo '✅ PASSED' || echo '❌ FAILED')"
echo "UI Tests:  $([ $UI_EXIT -eq 0 ] && echo '✅ PASSED' || echo '❌ FAILED')"
echo "-------------------------"

# Set exit code based on test results
if [ $API_EXIT -eq 0 ] && [ $UI_EXIT -eq 0 ]; then
    echo "All tests PASSED!"
    exit 0
else
    echo "Some tests FAILED. Check logs for details."
    exit 1
fi
