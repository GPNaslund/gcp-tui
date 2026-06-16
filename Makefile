.PHONY: build test vet install clean

build:
	go build -o gcp-tui .

test:
	go test ./...

vet:
	go vet ./...

install:
	go install .

clean:
	rm -f gcp-tui
