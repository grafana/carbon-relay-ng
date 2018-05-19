
Aggregation
-----------

As discussed in concepts above, we can combine, at each point in time, the points of multiple series into a new series.
Note:
* The interval parameter let's you quantize ("fix") timestamps, for example with an interval of 60 seconds, if you have incoming metrics for times that differ from each other, but all fall within the same minute, they will be counted together.
* The wait parameter allows up to the specified amount of seconds to wait for values, With a wait of 120, metrics can come 2 minutes late and still be included in the aggregation results.
* The fmt parameter dictates what the metric key of the aggregated metric will be.  use $1, $2, etc to refer to groups in the regex
  Multi-value aggregators (currently only percentiles) add .pxx at the end of the various metrics they emit.
  Single-value aggregators (currently all others) don't, allowing you to specify keywords like avg, sum, etc wherever into the fmt string you want.
* Note that we direct incoming values to an aggregation bucket based on the interval the timestamp is in, and the output key it generates.
  This means that you can have 3 aggregation cases, based on how you set your regex, interval and fmt string.
  - aggregation of points with different metric keys, but with the same, or similar timestamps) into one outgoing value (~ carbon-aggregator).
    if you set the interval to the period between each incoming packet of a given key, and the fmt yields the same key for different input metric keys
  - aggregation of individual metrics, i.e. packets for the same key, with different timestamps.  For example if you receive values for the same key every second, you can aggregate into minutely buckets by setting interval to 60, and have the fmt yield a unique key for every input metric key.  (~ graphite rollups)
  - the combination: compute aggregates from values seen with different keys, and at multiple points in time.
* functions currently available: avg, delta, derive, last, max, min, stdev, sum, percentiles
* aggregation output is routed via the routing table just like all other metrics.  Note that aggregation output will never go back into aggregators (to prevent loops) and also bypasses the validation and blacklist and rewriters.
* see the included ini for examples
* each aggregator can be configured to cache regex matches or not. there is no cache size limit because a limited size, under a typical workload where we see each metric key sequentially, in perpetual cycles, would just result in cache thrashing and wasting memory. If enabled, all matches are cached for at least 100 times the wait parameter. By default, the cache is enabled for aggregators set up via commands (init commands in the config) but disabled for aggregators configured via config sections (due to a limitation in our config library).  Basically enabling the cache means you trade in RAM for cpu.
* each aggregator may have a prefix and/or substring specified, these are used to reduce overhead by pre-filtering the metrics before they are matched against the regex (if not specified the prefix will be derived from the regex where possible).
* an aggregator may be configured with `dropRaw=true`, which will prevent any further processing of the raw series "consumed" by that aggregator (including by other aggregators).  This can be useful for managing cardinality and for quantizing metrics sent at odd intervals.  When using `dropRaw` an aggregator may produce a series with the same name as the input series. Note that this option may slow down table processing, especially with a cold or disable aggregator cache.


