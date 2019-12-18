# how to use:

# start metrictank docker-dev environment

* use a recent version of metrictank that has [this pr](https://github.com/grafana/metrictank/pull/1299) merged so you have 1s-resolution cpu stats of the host, containers, etc
* use docker-dev mt environment, but change scripts/config/storage-schemas.conf to:

````
[relay]
pattern = .*carbon-relay-ng.*
retentions = 1s:35d:10min:7 

[default]
pattern = .*
retentions = 10s:35d:10min:7
# reorderBuffer = 20
````

* import latest carbon-relay-ng dashboard (from crng repo) into grafana at localhost

# start the relay

in crng repo:

```
make
./carbon-relay-ng experiments/issue-241/carbon-relay-ng.ini > out.txt 2> err.txt
```

# start fakemetrics


```
fakemetrics --statsd-addr localhost:8125 feed --carbon-addr localhost:2002 --mpo 20000
```

find an mpo number that works for you. you want the relay to use about 70~110% CPU
note that docker-dev has a fakemetrics dashboard to see the stats of the fakemetrics process itself

# what's happening


a bunch of cnt aggregations each captures 10 points of each series (and emit a key across all input series), whereas a few others generate an output metric for each input metric
even without count-global, flush takes way too long

* using the dashboard-aggregation-analyzer.json dashboard and the new carbon-relay-ng dashboard you can analyze whether points were dropped, not accounted for, or incorrect.
* note the relay prints the aggregator keys at startup. these are used in some of the new metrics in the dashboard.

In dieter's experience so far, trying to reproduce https://github.com/grafana/carbon-relay-ng/issues/241

* once cpu usage gets above 50~80%, ingestion rate of the relay becomes non steady (no longer a flatline). this is without adjusting fakemetrics, and with fakemetrics reporting a steady output stream.  this can be reproduced by restarting the relay with more or less of the aggregations enabled. it seems when this happens, aggregators are more likely to miss input data. (see new inbound metrics). ingestion rate should always be nice and stable
* sometimes outbound rate of an aggregator seems to go down whereas inbound was a solid flatline. but you need to be patient to see this happen. sometimes it takes hours. have not seen correlation with any of the containers nor the host taking extra cpu.
