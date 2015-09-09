#!/bin/sh

export VERSION=master
export GOPATH=/

echo "Building carbon-relay-ng revision $revision"
docker pull gliderlabs/alpine
docker run --rm -v $(pwd):/export gliderlabs/alpine /bin/sh -c "\
  apk --update add git go make && \
  export GOPATH=$GOPATH && \
  export PATH="$PATH:$GOPATH/bin" && \
	go get -d github.com/graphite-ng/carbon-relay-ng && \
	go get github.com/jteeuwen/go-bindata/... && \
	cd "$GOPATH/src/github.com/graphite-ng/carbon-relay-ng" && \
	git checkout $VERSION && \
	make && \
	cp carbon-relay-ng /export"

echo "docker build --tag=carbon-relay-ng:latest ."
docker build --tag=carbon-relay-ng:latest .
