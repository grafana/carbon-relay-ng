# Installation

## Release vs latest master

Typically, some new code was added to master since the last release.
See the [changelog](https://github.com/grafana/carbon-relay-ng/blob/master/CHANGELOG.md) or look at the [git log](https://github.com/grafana/carbon-relay-ng/commits/master) for the most up to date information.
This may help you decide whether you want the latest release, or the latest code.

## Linux distribution packages

Grafana Labs provides 2 repositories for carbon-relay-ng:

* [raintank](https://packagecloud.io/raintank/raintank): stable repository for official stable releases
* [testing](https://packagecloud.io/raintank/testing): testing repository that has the latest packages which typically bring improvements but possibly also new bugs.

See the installation instructions on those pages for how to enable the repositories for your distribution

We host packages for Ubuntu 14.04 (trusty), 16.04 (xenial), debian 8, 9, 10 (jessie, stretch and buster/testing), Centos6 and Centos7.

## Binaries

Executable Binaries for Linux, Mac, and Windows can be found on the [releases](https://github.com/grafana/carbon-relay-ng/releases) page (starting with v0.13.0) .

## Docker images

See [dockerhub](https://hub.docker.com/r/grafana/carbon-relay-ng/).

You can use these tags:

* `latest`: the latest official stable release
* `master`: latest build from master. these versions typically bring improvements but possibly also new bugs


# Building from source

Requires Go 1.7 or higher.
We use [dep](https://golang.github.io/dep/) to manage vendoring 3rd party libraries
These commands will install the binary as `$GOPATH/bin/carbon-relay-ng`

    export GOPATH=/some/path/
    export PATH="$PATH:$GOPATH/bin"
    go get github.com/grafana/carbon-relay-ng/...

If you want to make your own changes and rebuild:

    cd "$GOPATH/src/github.com/grafana/carbon-relay-ng"
    # e.g. to check out an older version:
    # git checkout 0.10.1
    go get github.com/shuLhan/go-bindata/cmd/go-bindata
    make


This leaves you with a binary that you can run with a config file like so:

```
./carbon-relay-ng -h
Usage:
        carbon-relay-ng version
        carbon-relay-ng <path-to-config>
	
  -block-profile-rate int
    	see https://golang.org/pkg/runtime/#SetBlockProfileRate
  -cpuprofile string
    	write cpu profile to file
  -mem-profile-rate int
    	0 to disable. 1 for max precision (expensive!) see https://golang.org/pkg/runtime/#pkg-variables (default 524288)
```


# Release process

During normal development, maintain CHANGELOG.md, and mark interesting -to users- changes under "unreleased" version.
Grafana Labs regularly deploys the latest code from `master`, but cannot possibly do extensive testing of all functionality in production, so users are encouraged to run master also, and report any issues they hit.
When interesting changes have been merged to master, and they have had a chance to be tested for a while, we tag a release, as follows:

* update CHANGELOG.md from `unreleased` to the version
* create annotated git tag in the form `v<version>` and push to GitHub
* trigger re-run of the CI pipeline to build packages with new version tag
* wait for CircleCI to complete successfully
* create release on GitHub. copy entry from CHANGELOG.md to GitHub release page
* upload platform specific (Linux, Mac and windows) binaries from CircleCI to the release as well
