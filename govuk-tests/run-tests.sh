#!/bin/bash

# This tests starts the carbon-relay container and a test-receiver container to
# mimic stunnel.  The test-receiver sends a metric to carbon-relay and then
# waits for it to send back the correctly formatted metric as carbon-relay
# should send to stunnel. If no metric is received or the format is incorrect
# the test fails.

echo "Running tests"

echo "Starting containers"

if docker-compose up --build --exit-code-from test-receiver; then
  echo "Pass"
else
  echo "Fail"
  exit 1
fi
