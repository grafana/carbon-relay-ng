The GrafanaNet route is a special route within carbon-relay-ng.

It takes graphite (carbon) input and submits it to a Grafana Cloud metrics store in encrypted form.
There are two files you can provide to this route: storage-schemas.conf (required) and storage-aggregation.conf (optional).
On Grafana Cloud Graphite v5, these files are used to render data and generate your rollups.
Typically, you will have these files already set up if you use Graphite.
If the storage-aggregation.conf file is not provided, the default Grafana Cloud aggregations will be used when rendering data.

See Graphite docs for [storage-schemas.conf](http://graphite.readthedocs.io/en/latest/config-carbon.html#storage-schemas-conf) and [storage-aggregation.conf from Graphite](https://graphite.readthedocs.io/en/latest/config-carbon.html#storage-aggregation-conf).

(Note that unlike Graphite, you may change these files as necessary to describe your data and desired rollups.
After a carbon-relay-ng restart they will take effect immediately, even on historical data without having to run any data conversion.)


**Note: For more details on GrafanaCloud, head over to https://grafana.com/cloud/metrics**

# The quick way

The easiest way is to [create an instance on Grafana Cloud](https://grafana.com/products/cloud/), go to your instance details page, and follow the instructions there.
The config there is the best starting point.

# The more detailed way

[Config docs for GrafanaNet route](config.md#grafananet-route)

note:
* it requires a grafana.com api key and a url for ingestion, which will be shown on the instance details page in your Grafana Cloud portal.
* api key should have editor or admin role.
* it needs to read your graphite storage-schemas.conf (and optionally, storage-aggregation.conf) as described above.
* any metric messages that don't validate are filtered out. see the admin ui to troubleshoot if needed.
* by specifying a prefix, sub or regex you can only send a subset of your metrics to Grafana Cloud Graphite.

See [Grafana Cloud data ingestion docs](https://grafana.com/docs/grafana-cloud/metrics/graphite/data-ingestion/) for more information.

