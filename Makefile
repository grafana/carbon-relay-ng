VERSION=$(shell git describe --tags --always | sed 's/^v//')


build:
	cd ui/web && go-bindata -pkg web admin_http_assets
	find . -name '*.go' | grep -v '^\.\/vendor' | xargs gofmt -w -s
	CGO_ENABLED=0 go build ./cmd/carbon-relay-ng

docker: build
	docker build --tag=raintank/carbon-relay-ng:latest .

all:

deb: deb-systemd

deb-systemd: build deb-common
	mkdir -p build/deb-systemd
	install -d debian/lib/systemd/system
	install examples/carbon-relay-ng.service debian/lib/systemd/system
	DEBDIR=build/deb-systemd $(MAKE) deb-fpm

deb-upstart: build deb-common
	mkdir -p build/deb-upstart
	DEBDIR=build/deb-upstart FPMARGS="--deb-upstart examples/carbon-relay-ng.upstart" $(MAKE) deb-fpm

deb-nostart: build deb-common
	mkdir -p build/deb-nostart
	DEBDIR=build/deb-nostart $(MAKE) deb-fpm

deb-fpm:
	fpm $(FPMARGS) \
		-s dir \
		-t deb \
		-n carbon-relay-ng \
		-v $(VERSION)-1 \
		-a native \
		-p $(DEBDIR)/carbon-relay-ng-VERSION_ARCH.deb \
		-m "Dieter Plaetinck <dieter@raintank.io>" \
		--description "Fast carbon relay+aggregator with admin interfaces for making changes online" \
		--license BSD \
		--url https://github.com/graphite-ng/carbon-relay-ng \
		-C debian .
	rm -rf debian


deb-common:
	install -d debian/etc/carbon-relay-ng
	install examples/carbon-relay-ng.ini debian/etc/carbon-relay-ng/carbon-relay-ng.conf
	install -d debian/var/run/carbon-relay-ng
	install -d debian/usr/bin
	install carbon-relay-ng debian/usr/bin
	install -d debian/usr/share/man/man1
	install man/man1/carbon-relay-ng.1 debian/usr/share/man/man1
	gzip debian/usr/share/man/man1/carbon-relay-ng.1
	install -d debian/usr/share/doc/carbon-relay-ng/examples
	install examples/* debian/usr/share/doc/carbon-relay-ng/examples

rpm: build
	mkdir -p build/centos-7
	install -d redhat/usr/bin redhat/usr/share/man/man1 redhat/etc/carbon-relay-ng redhat/lib/systemd/system redhat/var/run/carbon-relay-ng
	install carbon-relay-ng redhat/usr/bin
	install man/man1/carbon-relay-ng.1 redhat/usr/share/man/man1
	install examples/carbon-relay-ng.ini redhat/etc/carbon-relay-ng/carbon-relay-ng.conf
	install examples/carbon-relay-ng.service redhat/lib/systemd/system
	gzip redhat/usr/share/man/man1/carbon-relay-ng.1
	fpm \
		-s dir \
		-t rpm \
		-n carbon-relay-ng \
		-v $(VERSION) \
		--epoch 1 \
		-a native \
		-p build/centos-7/carbon-relay-ng-VERSION.el7.ARCH.rpm \
		-m "Dieter Plaetinck <dieter@raintank.io>" \
		--description "Fast carbon relay+aggregator with admin interfaces for making changes online" \
		--license BSD \
		--url https://github.com/graphite-ng/carbon-relay-ng \
		-C redhat .
	rm -rf redhat	

rpm-centos6: build
	mkdir build/centos-6
	install -d redhat/usr/bin redhat/usr/share/man/man1 redhat/etc/carbon-relay-ng redhat/etc/init
	install carbon-relay-ng redhat/usr/bin
	install man/man1/carbon-relay-ng.1 redhat/usr/share/man/man1
	install examples/carbon-relay-ng.ini redhat/etc/carbon-relay-ng/carbon-relay-ng.conf
	install examples/carbon-relay-ng.upstart-0.6.5 redhat/etc/init/carbon-relay-ng.conf
	gzip redhat/usr/share/man/man1/carbon-relay-ng.1
	fpm \
		-s dir \
		-t rpm \
		-n carbon-relay-ng \
		-v $(VERSION) \
		--epoch 1 \
		-a native \
		-p build/centos-6/carbon-relay-ng-VERSION.el6.ARCH.rpm \
		-m "Dieter Plaetinck <dieter@raintank.io>" \
		--description "Fast carbon relay+aggregator with admin interfaces for making changes online" \
		--license BSD \
		--url https://github.com/graphite-ng/carbon-relay-ng \
		-C redhat .
	rm -rf redhat

packages: deb deb-upstart rpm rpm-centos6

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
	docker run --rm -p 2003:2003 -p 2004:2004 -p 8081:8081 -v $(pwd)/examples:/conf -v $(pwd)/spool:/spool raintank/carbon-relay-ng

.PHONY: all deb gh-pages install man
