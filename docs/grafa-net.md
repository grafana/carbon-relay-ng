The GrafanaNet route is a special route within carbon-relay-ng.

It converts graphite (carbon) input into metrics2.0 form and submits it to a raintank metrics store.
**Note: the hosted metrics store is currently not yet a public service.  It can currently only be obtained on demand and for proof of concepts. (get in touch at hello@raintank.io if interested).**

note:
* it requires a grafana.net api key and a url to the hosted store
* it needs to read your graphite storage-schemas.conf to determine which intervals to use
* but will send your metrics the way you send them.
  (if you send at odd intervals, or an interval that doesn't match your storage-schemas.conf, that's how we will store it)
* any metric messages that don't validate are filtered out. see the admin ui to troubleshoot if needed.


You can use a configuration like the one below:
(note the double space to separate the route definiton from the endpoint properties)
```
instance = "proxy"

max_procs = 2

listen_addr = "0.0.0.0:2013"
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
     'addRoute grafanaNet grafanaNet  http://localhost:8081/metrics your-grafana.net-api-key /path/to/storage-schemas.conf',
]

[instrumentation]
# in addition to serving internal metrics via expvar, you can optionally send em to graphite
graphite_addr = "localhost:2003"
graphite_interval = 1000  # in ms
```

## performance tuning

You can tune the flushing behavior.
Default values:
* bufSize: 1e7 (10 million)
* flushMaxNum: 10k
* flushMaxWait: 500 (milliseconds)

You want to have it flush "early" so your data shows up quickly, but since currently there's only one worker
you have to check the carbon-relay-ng dashboard (or the log) to make sure flushes don't last so long that the buffer fills up.
In that case, increase the flush values, so that we can flush more data per flush than what comes in.

You can pass extra space-separated arguments (after the storage-schemas.conf path)  like so:
```
bufSize=1000000 flushMaxNum=5000 flushMaxWait=1000
```


