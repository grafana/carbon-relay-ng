
Aggregation
-----------

Aggregation is used to combine data together via aggregation functions, either across time, across series, or across both.
Aggregators can also be (ab)used for quantizing (normalizing timestamps) or renaming series (use a rewriter instead).

## input

any data matching the provided regex is routed into the aggregator.
each aggregator may have a prefix and/or substring specified, these are used to reduce overhead by pre-filtering the metrics before they are matched against the regex (if not specified the prefix will be derived from the regex where possible).

## bucketing

incoming data is bucketed in a bucket per:
* output key (which is specified from the format, and can optionally incorporate pieces from the input key via the regex groups,
  so you get flexibility to unite different series into the same bucket, or groups of separate buckets)
* output timestamp, which is qantized via the interval setting
  (for example with interval=60, input data with timestamps 60001, 60010, 60020, 60030, 60059 will all get timestamp 60000)

The output of the aggregation bucket (after the wait timer expires) is then 1 point, aggregated across all input data for that bucket.

## functions

Available functions: 
function       | output
---------------|----------------------------------------------
avg            | average (mean)
delta          | difference between highest and lowest value seen
derive         | derivative (needs at least 2 input values. if more, derives from oldest to newest)
last           | last value seen in the bucket
max            | max value seen in the bucket
min            | min value seen in the bucket
stdev          | standard devation
sum            | sum
percentiles    | a set of different percentiles

## configuration


* The wait parameter allows up to the specified amount of seconds to wait for values:
With a wait of 120, metrics can come 2 minutes late and still be included in the aggregation results.
* The fmt parameter dictates what the metric key of the aggregated metric will be.  use $1, $2, etc to refer to groups in the regex
  Multi-value aggregators (currently only percentiles) add .pxx at the end of the various metrics they emit.
  Single-value aggregators (currently all others) don't, allowing you to specify keywords like avg, sum, etc wherever into the fmt string you want.
* Note that we direct incoming values to an aggregation bucket based on the interval the timestamp is in, and the output key it generates.
  This means that you can have 3 aggregation cases, based on how you set your regex, interval and fmt string.
  - aggregation of points with different metric keys, but with the same, or similar timestamps) into one outgoing value (~ carbon-aggregator).
    if you set the interval to the period between each incoming packet of a given key, and the fmt yields the same key for different input metric keys
  - aggregation of individual metrics, i.e. packets for the same key, with different timestamps.  For example if you receive values for the same key every second, you can aggregate into minutely buckets by setting interval to 60, and have the fmt yield a unique key for every input metric key.  (~ graphite rollups)
  - the combination: compute aggregates from values seen with different keys, and at multiple points in time.

* `dropRaw=true` will prevent any further processing of the raw series "consumed" by that aggregator (including by other aggregators).  This can be useful for managing cardinality and for quantizing metrics sent at odd intervals.  When using `dropRaw` an aggregator may produce a series with the same name as the input series. Note that this option may slow down table processing, especially with a cold or disable aggregator cache.

[config examples](https://github.com/graphite-ng/carbon-relay-ng/blob/master/docs/config.md#aggregators)

## output

Aggregation output is routed via the routing table just like all other metrics.
Note that aggregation output will never go back into aggregators (to prevent loops) and also bypasses the validation and blacklist and rewriters.

## caching

each aggregator can be configured to cache regex matches or not. there is no cache size limit because a limited size, under a typical workload where we see each metric key sequentially, in perpetual cycles, would just result in cache thrashing and wasting memory. If enabled, all matches are cached for at least 100 times the wait parameter. By default, the cache is enabled for aggregators set up via commands (init commands in the config) but disabled for aggregators configured via config sections (due to a limitation in our config library).  Basically enabling the cache means you trade in RAM for cpu.


