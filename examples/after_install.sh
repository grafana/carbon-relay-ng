#!/bin/sh


setup_carbon_relay_user() {
    if ! getent passwd carbon-relay-ng >/dev/null; then
        useradd --system --user-group --no-create-home --home /var/run/carbon-relay-ng --shell /usr/sbin/nologin carbon-relay-ng
    fi
}


# Create a carbon-relay-ng user if it does not already exists
setup_carbon_relay_user
