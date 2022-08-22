# GOV.UK carbon-relay-ng tests

The tests in this directory are only to validate the built container image can successfully run carbon-relay-ng.

We are deviating from the official grafana/carbon-relay-ng docker image, so we are explicitly testing our changes and trusting the upstream build and test otherwise.

To run the tests you first need to build carbon-relay-ng, and then you can run the `run-tests.sh` script.
