VERSION=$(shell git describe --tags --always | sed 's/^v//')
export GO111MODULE := on

# Run e.g. "make LINUX_PACKAGE_GOARCH=arm64" to produce ARM64 packages
LINUX_PACKAGE_GOARCH ?= amd64

build:
	cd ui/web && go-bindata -pkg web admin_http_assets/...
	find . -name '*.go' | grep -v '^\.\/vendor' | xargs gofmt -w -s
	CGO_ENABLED=0 go build -ldflags "-X main.Version=$(VERSION)" ./cmd/carbon-relay-ng

build-win: carbon-relay-ng.exe

carbon-relay-ng.exe:
	cd ui/web && go-bindata -pkg web admin_http_assets/...
	find . -name '*.go' | grep -v '^\.\/vendor' | xargs gofmt -w -s
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.Version=$(VERSION)" -o carbon-relay-ng.exe ./cmd/carbon-relay-ng

build-darwin: carbon-relay-ng-darwin

carbon-relay-ng-darwin:
	cd ui/web && go-bindata -pkg web admin_http_assets/...
	find . -name '*.go' | grep -v '^\.\/vendor' | xargs gofmt -w -s
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.Version=$(VERSION)" -o carbon-relay-ng-darwin ./cmd/carbon-relay-ng

build-linux: carbon-relay-ng-linux-$(LINUX_PACKAGE_GOARCH) build/bin/carbon-relay-ng-linux-$(LINUX_PACKAGE_GOARCH)

build-bsd:
	cd ui/web && go-bindata -pkg web admin_http_assets/...
	find . -name '*.go' | grep -v '^\.\/vendor' | xargs gofmt -w -s
	GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.Version=$(VERSION)" -o carbon-relay-ng-bsd ./cmd/carbon-relay-ng

carbon-relay-ng-linux-%:
	cd ui/web && go-bindata -pkg web admin_http_assets/...
	find . -name '*.go' | grep -v '^\.\/vendor' | xargs gofmt -w -s
	GOOS=linux GOARCH=$* CGO_ENABLED=0 go build -ldflags "-X main.Version=$(VERSION)" -o $@ ./cmd/carbon-relay-ng

build/bin/carbon-relay-ng-linux-%: carbon-relay-ng-linux-%
	mkdir -p build/bin
	cp carbon-relay-ng-linux-$* $@

test:
	go test -v -race ./...

print-version:
	@echo $(VERSION)

docker: build-linux
	docker build -t carbon-relay-ng:$(VERSION) .

docker-k8s: build-linux
	cp build/bin/* operations/k8s/
	docker build -t carbon-relay-ng-k8s:$(VERSION) operations/k8s

all:

gh-pages: man
	mkdir -p gh-pages
	find man -name \*.html | xargs -I__ mv __ gh-pages/
	git checkout -q gh-pages
	cp -R gh-pages/* ./
	rm -rf gh-pages
	git add .
	git commit -m "Rebuilt manual."
	git push origin gh-pages
	git checkout -q main

install: build
	go install

man:
	find man -name \*.ronn | xargs -n1 ronn --manual=carbon-relay-ng --style=toc

run: build
	./carbon-relay-ng carbon-relay-ng.ini

run-docker:
	docker run --rm -p 2003:2003 -p 2004:2004 -p 8081:8081 -v $(CURDIR)/examples:/conf -v $(CURDIR)/spool:/spool grafana/carbon-relay-ng

clean:
	rm -f carbon-relay-ng carbon-relay-ng.exe

packages-minor-autoupdate:
	go mod edit -json \
		| jq ".Require \
			| map(select(.Indirect | not).Path) \
			| map(select( \
				. != \"github.com/BurntSushi/toml\" \
			))" \
		| tr -d '\n' | tr -d '  '

.PHONY: all gh-pages install man test build clean build-linux packages-minor-autoupdate
