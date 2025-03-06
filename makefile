# COre make file
# runs go backend
# runs react frontend

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