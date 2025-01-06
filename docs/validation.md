# Validation

Incoming metrics are validated in multiple steps:

1. for pickle: protocol-level checks
2. message validation
3. order validation

Invalid metrics are dropped and - provided the message could be parsed - can be seen at /badMetrics/timespec.json where timespec is something like 30s, 10m, 24h, etc.
Carbon-relay-ng exports counters for invalid and out of order metrics (see [monitoring](https://github.com/grafana/carbon-relay-ng/blob/main/docs/monitoring.md))

Let's clarify step 2 and 3.

## Message validation

We first validate:

* the message has 3 fields with a non-empty key
* the value parses to an int or float
* the timestamp is a unix timestamp

Validation of the keys is a bit trickier, because the graphite project doesn't clearly specify what is valid and what is not.
So carbon-relay-ng provides a few options.
First of all, if the key contains `=` or `_is_` we validate the key as metric2.0, otherwise as a standard carbon metric.

#### standard carbon key

| Level            | Description                                                                                                                     |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| none             | no validation                                                                                                                   |
| medium (default) | ensure characters are 8-bit clean and not NULL. Optional tag appendix: see below                                                |
| strict           | medium + before appendix: block anything that can upset graphite: valid characters are `[A-Za-z0-9_-.]` and no consecutive dots |


###### tag appendix validation

The tag appendix is more clearly defined [in the graphite docs](https://graphite.readthedocs.io/en/latest/tags.html)
The rules are :
* to separate tags from each other and from the metric name: `;`
* each tag is a non-empty key and value string, separated by `=`. Keys and values may not contain `;`. The key may not contain `!`.
We also add the 8-bit cleanness and not-NULL check on the appendix

Can be changed with `legacy_metric_validation` configuration parameter

#### metrics2.0

| Level            | Description                                                                |
| ---------------- | -------------------------------------------------------------------------- |
| none             | no validation                                                              |
| medium (default) | unit, mtype tag set. no mixing of `=` and `_is_` styles. at least two tags |
| strict           | reserved for future                                                        |


Can be changed with `validation_level_m20` configuration parameter

## Order validation

Rejects points if the timestamp is not newer than a previous point for the same metric key.
useful for some backends like grafana.net that may reject out-of-order data (based on configuration).

Default: disabled. enable with `validate_order` option.

