.PHONY: all linux darwin-arm64 clean

all: linux

linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -buildmode=c-shared -o libllmint-linux-amd64.so .

darwin-arm64:
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -buildmode=c-shared -o libllmint-darwin-arm64.dylib .

clean:
	rm -f libllmint-*.so libllmint-*.dylib libllmint-*.h
