#!/bin/sh

# Postinstall for packages published to the deprecated Packagecloud
# repositories. Identical to after_install.sh, but also warns about the
# Packagecloud deprecation. The apt.grafana.com / rpm.grafana.com packages use
# after_install.sh instead.

cat >&2 <<'EOF'
========================================================================
WARNING: this carbon-relay-ng package came from Packagecloud
(packagecloud.io/raintank/raintank), which is DEPRECATED and will be shut
down.

Switch to the Grafana Labs package repositories for future updates:
  APT: https://apt.grafana.com
  RPM: https://rpm.grafana.com

See https://github.com/grafana/carbon-relay-ng/blob/main/docs/installation-building.md
========================================================================
EOF

setup_carbon_relay_user() {
    if ! getent passwd carbon-relay-ng >/dev/null; then
        useradd --system --user-group --no-create-home --home /var/run/carbon-relay-ng --shell /usr/sbin/nologin carbon-relay-ng
    fi
}


# Create a carbon-relay-ng user if it does not already exists
setup_carbon_relay_user
