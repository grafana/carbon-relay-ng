#!/bin/sh
set -x

confd -onetime -backend env

exec "$@"
