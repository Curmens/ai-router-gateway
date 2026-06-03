BINARY = auto-router
CONFIG = configs/config.yaml

.PHONY: build run electron clean

build:
	cd application && npm install && npm run build
	go build -o $(BINARY) ./cmd/router

run: build
	./$(BINARY) --config $(CONFIG)

# Start the Go server then open the Electron window
electron: build
	./$(BINARY) --config $(CONFIG) &
	cd application && npx electron .

clean:
	rm -f $(BINARY)
	find internal/ui/dist -not -name 'index.html' -not -path 'internal/ui/dist' -delete
