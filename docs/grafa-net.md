The GrafanaNet route is a special route within carbon-relay-ng.

It converts graphite (carbon) input into metrics2.0 form and submits it to a raintank metrics store.
**Note: the hosted metrics store is currently not yet a public service.  It can currently only be obtained on demand and for proof of concepts. (get in touch at hello@raintank.io if interested).**

note:
* it requires a grafana.net api key and a url to the hosted store
* api key should have editor or admin role. (viewer works for now, but will be blocked in the future)
* it needs to read your graphite storage-schemas.conf to determine which intervals to use
* but will send your metrics the way you send them.
  (if you send at odd intervals, or an interval that doesn't match your storage-schemas.conf, that's how we will store it)
* any metric messages that don't validate are filtered out. see the admin ui to troubleshoot if needed.

## syntax

notes:
* the double space to separate the route definiton from the endpoint properties
* by specifying a prefix, sub or regex you can only send a subset of your metrics to grafana.net hosted metrics

```
addRoute GrafanaNet key [prefix/sub/regex]  addr apiKey schemasFile [spool=true/false sslverify=true/false bufSize=int flushMaxNum=int flushMaxWait=int timeout=int]")
```


### options

options can appear after the schemasFile, space-separated.

* for schemasFile, see [storage-schemas.conf documentation](http://graphite.readthedocs.io/en/latest/config-carbon.html#storage-schemas-conf)
* bufSize: 1e7 (10 million)
* flushMaxNum: after this many metrics have queued up, trigger a flush (default 10k)
* flushMaxWait: after this many milliseconds, trigger a flush (default 500)
* timeout: after how many milliseconds to consider a request to the hosted metrics to timeout, so that it will retry later (default 2000)
* sslverify: disables ssl verifications, useful for test/POC setups with self signed ssl certificates (default true)

Note that there's only 1 flush worker so you have to check the carbon-relay-ng dashboard (or the log)
to make sure flushes don't last so long that the buffer fills up.
In that case, increase the flush values, so that we can flush more data per flush.

## example configuration file

notes:
* the double space to separate the route definiton from the endpoint properties
* the instrumentation section sets up the relay to send its own performance metrics into itself. You can provide any graphite addr or an empty address to disable.

```
instance = "proxy"

max_procs = 2

listen_addr = "0.0.0.0:2003"
admin_addr = "0.0.0.0:2004"
http_addr = "0.0.0.0:8083"
spool_dir = "/var/spool/carbon-relay-ng"
#one of critical error warning notice info debug
log_level = "notice"
# How long to keep track of invalid metrics seen
# Useful time units are "s", "m", "h"
bad_metrics_max_age = "24h"

# put init commands here, in the same format as you'd use for the telnet interface
# here's some examples:
init = [
     'addRoute sendAllMatch carbon-default  your-actual-graphite-server:2003 spool=true pickle=false',
     'addRoute grafanaNet grafanaNet  http://localhost:8081/metrics your-grafana.net-api-key /path/to/storage-schemas.conf sslverify=false',
]

[instrumentation]
# in addition to serving internal metrics via expvar, you can optionally send em to graphite
graphite_addr = "localhost:2003"
graphite_interval = 1000  # in ms
```
