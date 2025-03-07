# Core make file
# Handles build, run, and testing commands for the algorithmic trading system

# Build the go backend
build:
	go build -o go-trader main.go

# Run the go backend
run: build
	./go-trader &

# Run the react frontend
dev-frontend:
	cd web-ui && npm run dev

# Run the go backend and react frontend
dev:
	go run main.go & make dev-frontend

# Cypress test commands
test-api:
	cd web-ui && npm run test:api

test-ui:
	cd web-ui && npm run test:ui

test-all:
	cd web-ui && npm run test:all

cypress-open:
	cd web-ui && npm run cypress

# Start servers and run tests (for CI and local testing)
# Windows-specific target
test-e2e-win:
	run-tests.bat

# Unix-specific target
test-e2e-unix:
	./run-tests.sh

# Detect OS and run appropriate test script
test-e2e:
	@echo "Detecting Operating System..."
	@if [ -n "$$(uname -s | grep -i Darwin)" ] || [ -n "$$(uname -s | grep -i Linux)" ]; then \
		make test-e2e-unix; \
	else \
		make test-e2e-win; \
	fi

# Start servers and open Cypress interactive mode
test-interactive:
	# Start backend and frontend
	go run main.go & \
	cd web-ui && npm run dev & \
	# Wait for servers to be ready
	sleep 10 && \
	# Open Cypress
	npm run cypress
