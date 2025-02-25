.PHONY: clean build dist

build:
	@echo "Building the project..."
	go build -o bin/loki-actor .
	@echo "Build complete."

clean:
	@echo "Cleaning up..."
	rm bin/loki-actor
	@echo "Clean complete."

dist:
	@echo "Creating docker distribution..."
	docker build -t loki-actor:latest .
	@echo "Docker distribution created."

