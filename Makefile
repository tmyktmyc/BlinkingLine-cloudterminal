.PHONY: build run test clean mock

BINARY=cloudterminal
VERSION ?= dev

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) .

run: build
	./$(BINARY)

mock: build
	./$(BINARY) --mock "demo:show me something cool" "test:write a hello world"

test:
	go test ./...

clean:
	rm -f $(BINARY)
