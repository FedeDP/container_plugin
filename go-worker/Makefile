all: lib

.PHONY: lib
lib:
	go build -v -o libworker.a -buildmode=c-archive .

.PHONY: exe
exe:
	go build -tags exe -v -o worker  .
