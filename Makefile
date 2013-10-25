all: build

clean:
	go clean
	rm -f folk.tar.gz

build:
	export GOBIN=$(shell pwd)
	go build

package: build
	tar -cvzf folk.tar.gz folk data/

test:
	go get -u -v
	go get github.com/knakk/specs
	go test -i
	go test ./...

integration: build
	./integration-tests.sh