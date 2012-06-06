VERSION=0.0.0
BUILD=1

all:

deb:
	go build
	install -d debian/usr/bin debian/usr/share/man/man1
	install carbon-relay-ng debian/usr/bin
	install man/man1/carbon-relay-ng.1 debian/usr/share/man/man1
	gzip debian/usr/share/man/man1/carbon-relay-ng.1
	fpm \
		-s dir \
		-t deb \
		-n carbon-relay-ng \
		-v $(VERSION)-$(BUILD) \
		-a amd64 \
		-m "Richard Crowley <r@rcrowley.org>" \
		--description "Route traffic to Graphite's carbon-cache.py." \
		--license BSD \
		--url https://github.com/rcrowley/carbon-relay-ng \
		-C debian .
	rm -rf debian

deploy:
	scp carbon-relay-ng_$(VERSION)-$(BUILD)_amd64.deb freight@packages.rcrowley.org:
	ssh -t freight@packages.rcrowley.org "freight add carbon-relay-ng_$(VERSION)-$(BUILD)_amd64.deb apt/precise && rm carbon-relay-ng_$(VERSION)-$(BUILD)_amd64.deb && freight cache apt/precise"

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

install:
	go install

man:
	find man -name \*.ronn | xargs -n1 ronn --manual=carbon-relay-ng --style=toc

.PHONY: all deb gh-pages install man
