
Installation
------------

You can install packages from the [raintank packagecloud repository](https://packagecloud.io/raintank/raintank)
We automatically build packages for Ubuntu 14.04 (trusty), 16.04 (xenial), debian 8, 9, 10 (jessie, stretch and buster/testing), Centos6 and Centos7 when builds in CircleCI succeed.
[Instructions for enabling the repository](https://packagecloud.io/raintank/raintank/install)

You can also just build a binary (see below) and run the binary with a config file like so:

<code>carbon-relay-ng [-cpuprofile <em>cpuprofile-file</em>] <em>config-file</em></code>


Building
--------

Requires Go 1.7 or higher.
We use https://github.com/kardianos/govendor to manage vendoring 3rd party libraries

    export GOPATH=/some/path/
    export PATH="$PATH:$GOPATH/bin"
    go get -d github.com/graphite-ng/carbon-relay-ng
    go get github.com/shuLhan/go-bindata/cmd/go-bindata
    cd "$GOPATH/src/github.com/graphite-ng/carbon-relay-ng"
    # optional: check out an older version: git checkout v0.5
    make


