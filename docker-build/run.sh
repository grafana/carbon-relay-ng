#!/bin/bash

# Run in the foreground with the example config. 
docker run \
  --rm \
  -p 2003:2003 \
  -p 2004:2004 \
  -p 8081:8081 \
  -v $(cd ../ && pwd):/conf \
  -v $(pwd)/spool:/spool \
  --entrypoint /bin/carbon-relay-ng \
  carbon-relay-ng \
  /conf/carbon-relay-ng.ini
