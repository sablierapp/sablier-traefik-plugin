build:
	go build -v .

test:
	go test .

lint:
	golangci-lint run

fmt:
	golangci-lint fmt
