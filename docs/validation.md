
Validation
----------

All incoming metrics undergo some basic sanity checks before the metrics go into the routing table.  We always check the following:

* the value parses to an int or float
* the timestamp is a unix timestamp

By default, we also apply the following checks to the metric name:

* has 3 fields
* the key has no characters beside `a-z A-Z _ - . =` (fairly strict but graphite causes problems with various other characters)
* has no empty node (like field1.field2..field4)

However, for legacy metrics, the `legacy_metric_validation` configuration parameter can be used to loosen the metric name checks. This can be useful when needing to forward metrics whose names you do not control.
The following are valid values for the `legacy_metric_validation` field:

* `strict` -- All checks described above are in force. This is the default.
* `medium` -- We validate that the metric name has only ASCII characters and no embedded NULLs.
* `none` -- No metric name checks are performed.

If we detect the metric is in metrics2.0 format we also check proper formatting, and unit and mtype are set.

Invalid metrics are dropped and can be seen at /badMetrics/timespec.json where timespec is something like 30s, 10m, 24h, etc.
(the counters are also exported.  See instrumentation section)

You can also validate that for each series, each point is older than the previous. using the validate_order option.  This is helpful for some backends like grafana.net

