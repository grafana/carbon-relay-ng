# Installation

## Release vs latest main

Typically, some new code was added to main since the last release.
See the [changelog](https://github.com/grafana/carbon-relay-ng/blob/main/CHANGELOG.md) or look at the [git log](https://github.com/grafana/carbon-relay-ng/commits/main) for the most up to date information.
This may help you decide whether you want the latest release, or the latest code.

## Linux distribution packages

Grafana Labs provides 2 repositories for carbon-relay-ng:

* [raintank](https://packagecloud.io/raintank/raintank): stable repository for official stable releases
* [testing](https://packagecloud.io/raintank/testing): testing repository that has the latest packages which typically bring improvements but possibly also new bugs.

See the installation instructions on those pages for how to enable the repositories for your distribution

We host packages for Ubuntu 14.04 (trusty), 16.04 (xenial), debian 8, 9, 10 (jessie, stretch and buster/testing), Centos6 and Centos7.

## Binaries

Executable Binaries for Linux, Mac, FreeBSD and Windows can be found on the [releases](https://github.com/grafana/carbon-relay-ng/releases) page (starting with v0.13.0) .

## Docker images

See [dockerhub](https://hub.docker.com/r/grafana/carbon-relay-ng/).

You can use these tags:

* `latest`: the latest official stable release
* `main`: latest build from main. these versions typically bring improvements but possibly also new bugs


# Building from source

Requires Go 1.7 or higher.
These commands will install the binary as `$GOPATH/bin/carbon-relay-ng`

    export GOPATH=$HOME/go
    export PATH="$PATH:$GOPATH/bin"
    mkdir -p $GOPATH/src/github.com/grafana
    cd $GOPATH/src/github.com/grafana/
    git clone https://github.com/grafana/carbon-relay-ng.git
    cd carbon-relay-ng
    # e.g. to check out a specific version instead of main:
    # git checkout v1.1
    go install github.com/shuLhan/go-bindata/cmd/go-bindata
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
Grafana Labs regularly deploys the latest code from `main`, but cannot possibly do extensive testing of all functionality in production, so users are encouraged to run main also, and report any issues they hit.
When interesting changes have been merged to main, and they have had a chance to be tested for a while, we tag a release, as follows:

* Update CHANGELOG.md from `unreleased` to the version. Create a PR and merge into the main branch. [Example PR](https://github.com/grafana/carbon-relay-ng/pull/512)
* Create a git tag for the new version from the CHANGELOG commit merged from the PR and push the tag to GitHub.
    Example for adding the v1.4.0 tag:
    ```
    git tag -a v1.4.0 -m "v1.4.0"
    git push origin v1.4.0
    ```
* Pushing the tag will automatically trigger the CI pipeline to build packages with new version tag.
* Release binaries will be appended to the Github release tag once CI completes successfully.
