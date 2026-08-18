.PHONY: build test clean

build:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o tgit .

test:
	go test ./...

clean:
	rm -f tgit
