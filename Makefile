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

build-linux: carbon-relay-ng-linux-$(LINUX_PACKAGE_GOARCH)

build-bsd:
	cd ui/web && go-bindata -pkg web admin_http_assets/...
	find . -name '*.go' | grep -v '^\.\/vendor' | xargs gofmt -w -s
	GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.Version=$(VERSION)" -o carbon-relay-ng-bsd ./cmd/carbon-relay-ng

carbon-relay-ng-linux-%:
	cd ui/web && go-bindata -pkg web admin_http_assets/...
	find . -name '*.go' | grep -v '^\.\/vendor' | xargs gofmt -w -s
	GOOS=linux GOARCH=$* CGO_ENABLED=0 go build -ldflags "-X main.Version=$(VERSION)" -o $@ ./cmd/carbon-relay-ng
	cp $@ carbon-relay-ng-linux

test:
	go test -v -race ./...

docker: build-linux
	./build_docker.sh

all:

deb: build-linux
	mkdir -p build/deb-systemd
	install -d debian/usr/bin debian/usr/share/man/man1 debian/etc/carbon-relay-ng debian/lib/systemd/system debian/var/run/carbon-relay-ng debian/usr/lib/tmpfiles.d
	install carbon-relay-ng-linux-$(LINUX_PACKAGE_GOARCH) debian/usr/bin/carbon-relay-ng
	install -m 0644 examples/carbon-relay-ng.ini debian/etc/carbon-relay-ng/carbon-relay-ng.conf
	install -m 0644 examples/carbon-relay-ng-tmpfiles.conf debian/usr/lib/tmpfiles.d/carbon-relay-ng.conf
	install -m 0644 examples/carbon-relay-ng.service debian/lib/systemd/system
	install -m 0644 man/man1/carbon-relay-ng.1 debian/usr/share/man/man1
	gzip debian/usr/share/man/man1/carbon-relay-ng.1
	fpm \
		-s dir \
		-t deb \
		-n carbon-relay-ng \
		-v $(VERSION)-1 \
		-a $(LINUX_PACKAGE_GOARCH) \
		--config-files etc/carbon-relay-ng/carbon-relay-ng.conf \
		-p build/deb-systemd/carbon-relay-ng-VERSION_ARCH.deb \
		-m "Dieter Plaetinck <dieter@grafana.com>" \
		--description "Fast carbon relay+aggregator with admin interfaces for making changes online" \
		--license BSD \
		--url https://github.com/grafana/carbon-relay-ng \
		--after-install examples/after_install.sh \
		-C debian .
	rm -rf debian

deb-upstart: build-linux
	mkdir -p build/deb-upstart
	install -d debian/usr/bin debian/usr/share/man/man1 debian/etc/carbon-relay-ng
	install carbon-relay-ng-linux-$(LINUX_PACKAGE_GOARCH) debian/usr/bin/carbon-relay-ng
	install -m 0644 examples/carbon-relay-ng.ini debian/etc/carbon-relay-ng/carbon-relay-ng.conf
	install -m 0644 man/man1/carbon-relay-ng.1 debian/usr/share/man/man1
	gzip debian/usr/share/man/man1/carbon-relay-ng.1
	fpm \
		-s dir \
		-t deb \
		-n carbon-relay-ng \
		-v $(VERSION)-1 \
		-a $(LINUX_PACKAGE_GOARCH) \
		--config-files etc/carbon-relay-ng/carbon-relay-ng.conf \
		-p build/deb-upstart/carbon-relay-ng-VERSION_ARCH.deb \
		--deb-upstart examples/carbon-relay-ng.upstart \
		-m "Dieter Plaetinck <dieter@grafana.com>" \
		--description "Fast carbon relay+aggregator with admin interfaces for making changes online" \
		--license BSD \
		--url https://github.com/grafana/carbon-relay-ng \
		-C debian .
	rm -rf debian

rpm: build-linux
	mkdir -p build/centos-7
	install -d redhat/usr/bin redhat/usr/share/man/man1 redhat/etc/carbon-relay-ng redhat/lib/systemd/system redhat/var/run/carbon-relay-ng redhat/etc/tmpfiles.d
	install carbon-relay-ng-linux-$(LINUX_PACKAGE_GOARCH) redhat/usr/bin/carbon-relay-ng
	install -m 0644 man/man1/carbon-relay-ng.1 redhat/usr/share/man/man1
	install -m 0644 examples/carbon-relay-ng.ini redhat/etc/carbon-relay-ng/carbon-relay-ng.conf
	install -m 0644 examples/carbon-relay-ng-tmpfiles.conf redhat/etc/tmpfiles.d/carbon-relay-ng.conf
	install -m 0644 examples/carbon-relay-ng.service redhat/lib/systemd/system
	gzip redhat/usr/share/man/man1/carbon-relay-ng.1
	fpm \
		-s dir \
		-t rpm \
		-n carbon-relay-ng \
		-v $(VERSION) \
		--epoch 1 \
		-a $(LINUX_PACKAGE_GOARCH) \
		--config-files etc/carbon-relay-ng/carbon-relay-ng.conf \
		-p build/centos-7/carbon-relay-ng-VERSION.el7.ARCH.rpm \
		-m "Dieter Plaetinck <dieter@grafana.com>" \
		--description "Fast carbon relay+aggregator with admin interfaces for making changes online" \
		--license BSD \
		--url https://github.com/grafana/carbon-relay-ng \
		--after-install examples/after_install.sh \
		-C redhat .
	rm -rf redhat

rpm-centos6: build-linux
	mkdir -p build/centos-6
	install -d redhat/usr/bin redhat/usr/share/man/man1 redhat/etc/carbon-relay-ng redhat/etc/init redhat/etc/init.d
	install carbon-relay-ng-linux-$(LINUX_PACKAGE_GOARCH) redhat/usr/bin/carbon-relay-ng
	install -m 0644 man/man1/carbon-relay-ng.1 redhat/usr/share/man/man1
	install -m 0644 examples/carbon-relay-ng.ini redhat/etc/carbon-relay-ng/carbon-relay-ng.conf
	install -m 0644 examples/carbon-relay-ng.upstart-0.6.5 redhat/etc/init/carbon-relay-ng.conf
	install examples/carbon-relay-ng.init redhat/etc/init.d/carbon-relay-ng
	gzip redhat/usr/share/man/man1/carbon-relay-ng.1
	fpm \
		-s dir \
		-t rpm \
		-n carbon-relay-ng \
		-v $(VERSION) \
		--epoch 1 \
		-a $(LINUX_PACKAGE_GOARCH) \
		--config-files etc/carbon-relay-ng/carbon-relay-ng.conf \
		-p build/centos-6/carbon-relay-ng-VERSION.el6.ARCH.rpm \
		-m "Dieter Plaetinck <dieter@grafana.com>" \
		--description "Fast carbon relay+aggregator with admin interfaces for making changes online" \
		--license BSD \
		--url https://github.com/grafana/carbon-relay-ng \
		-C redhat .
	rm -rf redhat

packages: packages-amd64 packages-arm64

packages-amd64:
	$(MAKE) LINUX_PACKAGE_GOARCH=amd64 deb deb-upstart rpm rpm-centos6

packages-arm64:
	$(MAKE) LINUX_PACKAGE_GOARCH=arm64 deb deb-upstart rpm rpm-centos6

gh-pages: man
	mkdir -p gh-pages
	find man -name \*.html | xargs -I__ mv __ gh-pages/
	git checkout -q gh-pages
	cp -R gh-pages/* ./
	rm -rf gh-pages
	git add .
	git commit -m "Rebuilt manual."
	git push origin gh-pages
	git checkout -q master

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

.PHONY: all deb gh-pages install man test build clean build-linux packages packages-amd64 packages-arm64
