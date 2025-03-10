.PHONY: clean build dist

build:
	@echo "Building the project..."
	CGO_ENABLED=0 go build -o bin/loki-actor .
	@echo "Build complete."

test:
	@echo "Running tests..."
	go test ./...
	@echo "Tests complete."

clean:
	@echo "Cleaning up..."
	rm bin/loki-actor
	@echo "Clean complete."

dist:
	@echo "Creating docker distribution..."
	docker build -t loki-actor:latest .
	@echo "Docker distribution created."

