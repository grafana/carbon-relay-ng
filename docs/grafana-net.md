The GrafanaNet route is a special route within carbon-relay-ng.

It converts graphite (carbon) input into metrics2.0 form and submits it to a grafanaCloud metrics store.
**Note: For more details on GrafanaCloud, head over to https://grafana.com/cloud/metrics**

# The quick way

The easiest way is to create an instance on Grafana Cloud, go to your instance details page, and follow the instructions there.
The config there is the best starting point.

# The more detailed way

[Config docs for GrafanaNet route](config.md#grafananet-route)

note:
* it requires a grafana.com api key and a url to the hosted store
* api key should have editor or admin role.
* it needs to read your graphite storage-schemas.conf to determine which intervals to use
* but will send your metrics the way you send them.
  (if you send at odd intervals, or an interval that doesn't match your storage-schemas.conf, that's how we will store it. so make sure you have this correct)
* any metric messages that don't validate are filtered out. see the admin ui to troubleshoot if needed.
* by specifying a prefix, sub or regex you can only send a subset of your metrics to grafana.com hosted metrics

See [Grafana Cloud data ingestion docs](https://grafana.com/docs/grafana-cloud/metrics/graphite/data-ingestion/) for more information.

