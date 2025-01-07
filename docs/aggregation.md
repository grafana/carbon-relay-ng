
Aggregation
-----------

Aggregation is used to reduce the volume of datapoints, across time, across series, or across both.
Data corresponding to the same "bucket" (see below) is aggregated using the configured aggregation function such as sum or average.
Aggregators can also be (ab)used for quantizing (normalizing timestamps) or renaming series (but better to use a rewriter instead).
Note that the original input data is still emitted, in addition to its aggregated series.  To only get the aggregator output and discard input metrics,
use `dropRaw = true` in aggregation config (see configuration section below).

## input

Any data that "matches" is routed into the aggregator.
Matching can be done using a prefix, substring and regular expression. They are optional, but incoming data must match all of the options that are set.
note: prefix and substring matching is a good way to reduce CPU usage, as it allows skipping regex matching for incoming data if it doesn't match the prefix or substring.
(if not specified the prefix will be derived from the regex where possible).

## bucketing

incoming data is grouped together into buckets, where each bucket corresponds to a combination of an output key and an output timestamp.

* output key (which is specified from the format, and can optionally incorporate pieces from the input key via the regex groups,
  so you get flexibility to unite different series into the same bucket, or groups of separate buckets)

  Example:
  ```
  regex = '^servers\.(dc[0-9]+)\.(app|proxy)[0-9]+\.(.*)'
  format = 'aggregates.$1.$2.$3.sum'
  ```

  If your incoming data has keys like:
  ```
  servers.dc1.app1.cpu_usage
  servers.dc1.app2.cpu_usage
  servers.dc1.app3.cpu_usage
  servers.dc1.proxy1.cpu_usage
  servers.dc1.proxy2.cpu_usage
  servers.dc1.proxy3.cpu_usage
  servers.dc2.proxy1.stats.num_requests
  servers.dc2.proxy2.stats.num_requests
  ```
  Then your outbound metrics will look like:
  ```
  aggregates.dc1.app.cpu_usage.sum
  aggregates.dc1.proxy.cpu_usage.sum
  aggregates.dc2.proxy.stats.num_requests.sum
  ```

* output timestamp, which is qantized via the interval setting
  (for example with interval=60, input data with timestamps 60001, 60010, 60020, 60030, 60059 will all get timestamp 60000)

The output of the aggregation bucket (after the wait timer expires) is then 1 point, aggregated across all input data for that bucket.

## functions

Available functions:

| function    | output                                                                             |
| ----------- | ---------------------------------------------------------------------------------- |
| avg         | average (mean)                                                                     |
| count       | number of points/values seen (count of items in the bucket)                        |
| delta       | difference between highest and lowest value seen                                   |
| derive      | derivative (needs at least 2 input values. if more, derives from oldest to newest) |
| last        | last value seen in the bucket                                                      |
| max         | max value seen in the bucket                                                       |
| min         | min value seen in the bucket                                                       |
| stdev       | standard devation                                                                  |
| sum         | sum                                                                                |
| percentiles | a set of different percentiles                                                     |

## configuration


* The wait parameter allows up to the specified amount of seconds to wait for values:
With a wait of 120, metrics can come 2 minutes after the start of the interval and still be included in the aggregation results.  The wait value should be set to the interval plus whatever the data delay is (time difference between timestamps of the data and the wall clock). For most environments the data delay is no more than a few seconds.
* The fmt parameter dictates what the metric key of the aggregated metric will be.  use $1, $2, etc to refer to groups in the regex (see "bucketing" above).
  Multi-value aggregators (currently only percentiles) add .pxx at the end of the various metrics they emit.
  Single-value aggregators (currently all others) don't, allowing you to specify keywords like avg, sum, etc wherever into the fmt string you want.
* Note that we direct incoming values to an aggregation bucket based on the interval the timestamp is in, and the output key it generates.
  This means that you can have 3 aggregation cases, based on how you set your regex, interval and fmt string.
  - aggregation of points with different metric keys, but with the same, or similar timestamps) into one outgoing value (~ carbon-aggregator).
    if you set the interval to the period between each incoming packet of a given key, and the fmt yields the same key for different input metric keys
  - aggregation of individual metrics, i.e. packets for the same key, with different timestamps.  For example if you receive values for the same key every second, you can aggregate into minutely buckets by setting interval to 60, and have the fmt yield a unique key for every input metric key.  (~ graphite rollups)
  - the combination: compute aggregates from values seen with different keys, and at multiple points in time.

* `dropRaw=true` will prevent any further processing of the raw series "consumed" by an aggregator with this option enabled.  It causes the original input series to disappear from the routing table.  This can be useful for managing cardinality and for quantizing metrics sent at odd intervals.  When using `dropRaw` an aggregator may produce a series with the same name as the input series. Note that this option may slow down table processing, especially with a cold or disabled aggregator cache.

[config examples](https://github.com/grafana/carbon-relay-ng/blob/main/docs/config.md#aggregators)

## output

Aggregation output is routed via the routing table just like all other metrics.
Note that aggregation output will never go back into aggregators (to prevent loops) and also bypasses the validation and blocklist and rewriters.

## caching

each aggregator can be configured to cache regex matches or not. there is no cache size limit because a limited size, under a typical workload where we see each metric key sequentially, in perpetual cycles, would just result in cache thrashing and wasting memory. If enabled, all matches are cached for at least 100 times the wait parameter. By default, the cache is enabled for aggregators set up via commands (init commands in the config) but disabled for aggregators configured via config sections (due to a limitation in our config library).  Basically enabling the cache means you trade in RAM for cpu.


