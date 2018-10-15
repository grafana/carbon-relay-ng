#!/bin/bash

if ! grep -q $GOPATH/bin <<< $PATH ; then
  export PATH="$PATH:$GOPATH/bin"
fi

if ! which dep >/dev/null; then
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
fi

dep version
dep status
dep check
