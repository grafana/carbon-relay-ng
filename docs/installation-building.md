# Installation

## Release vs latest main

Typically, some new code was added to main since the last release.
See the [changelog](https://github.com/grafana/carbon-relay-ng/blob/main/CHANGELOG.md) or look at the [git log](https://github.com/grafana/carbon-relay-ng/commits/main) for the most up to date information.
This may help you decide whether you want the latest release, or the latest code.

## Linux distribution packages

DEB and RPM packages (`amd64` and `arm64`) are published to Grafana Labs'
package repositories, [apt.grafana.com](https://apt.grafana.com) and
[rpm.grafana.com](https://rpm.grafana.com).

> **Deprecation:** the previous Packagecloud repositories
> (`packagecloud.io/raintank/raintank` and `raintank/testing`) are deprecated
> and will be shut down. If you installed from there, switch to the
> repositories below.

### APT (Debian, Ubuntu)

```sh
sudo apt-get install -y apt-transport-https wget gnupg
sudo mkdir -p /etc/apt/keyrings/
sudo wget -O /etc/apt/keyrings/grafana.asc https://apt.grafana.com/gpg-full.key
sudo chmod 644 /etc/apt/keyrings/grafana.asc
echo "deb [signed-by=/etc/apt/keyrings/grafana.asc] https://apt.grafana.com stable main" | sudo tee -a /etc/apt/sources.list.d/grafana.list
sudo apt-get update
sudo apt-get install carbon-relay-ng
```
### RPM (RHEL, CentOS, Fedora)

```sh
wget -q -O gpg.key https://rpm.grafana.com/gpg.key
sudo rpm --import gpg.key
sudo tee /etc/yum.repos.d/grafana.repo > /dev/null <<'EOF'
[grafana]
name=grafana
baseurl=https://rpm.grafana.com
repo_gpgcheck=1
enabled=1
gpgcheck=1
gpgkey=https://rpm.grafana.com/gpg.key
sslverify=1
sslcacert=/etc/pki/tls/certs/ca-bundle.crt
EOF
sudo dnf install carbon-relay-ng
```
## Binaries

Executable Binaries for Linux, Mac, FreeBSD and Windows can be found on the [releases](https://github.com/grafana/carbon-relay-ng/releases) page (starting with v0.13.0) .

## Docker images

See [dockerhub](https://hub.docker.com/r/grafana/carbon-relay-ng/).

You can use these tags:

* `<version>` (e.g. `1.5.14`): an official stable release. Pin to one of these for reproducible deployments.
* `main-<short-sha>`: an immutable build from the corresponding commit on the `main` branch. These typically bring improvements but possibly also new bugs.

> **Note:** the mutable `latest`, `main` and `master` tags are no longer updated and are frozen at their last published value. Use an immutable `main-<short-sha>` tag or pin to a release version instead.


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
