#!/bin/bash
version=$(git describe --tags --always | sed 's/^v//')

# only tag as latest if we're in master branch and the version tag has no hyphen in it.
# note: we may want to extend this to also not tag as latest if working tree is dirty.
# but i think because of how go bindata works, it probably makes a change in the working tree.
tag=master
[[ "$GITHUB_EVENT_NAME" = "push" ]] && [[ "$GITHUB_REF_TYPE" = "tag" ]] && [[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] && tag=latest


docker build --tag=grafana/carbon-relay-ng:$tag .
docker tag grafana/carbon-relay-ng:$tag grafana/carbon-relay-ng:$version
