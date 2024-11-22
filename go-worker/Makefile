all: lib

.PHONY: lib
lib:
	go build -v -o libworker.a -buildmode=c-archive .

.PHONY: exe
exe:
	go build -tags exe -v -o worker  .

clean:
	rm -rf worker libworker.a libworker.h

.PHONY: test
test:
	go clean -testcache
	GOEXPERIMENT=loopvar go test -v -cover -race ./...