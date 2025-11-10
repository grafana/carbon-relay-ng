# unreleased

# v1.5.9: November 10, 2025
* Bump go version to 1.24.9
* Various minor dependency updates

# v1.5.8: August 12, 2025
* Bump go version to 1.24.6

# v1.5.7: July  21, 2025
* Trying to diagnose docker deploy failure

# v1.5.6: July  21, 2025
* Bump go toolchain to 1.24.4

# v1.5.5: May 21, 2025
* Migrate to `aws-sdk-go-v2`

# v1.5.4: May 19, 2025
* Bump go toolchain to 1.24.2

# v1.5.3: March 31, 2025
* Bump go to 1.24
* Various minor dependency updates
* Fix FlushSize metric in the grafananetrouter #582

# v1.5.2: January 6, 2025
* Various minor dependency updates

# v1.5.1: December 5, 2024
* Use distroless docker image #571
* Bump go to 1.23.4 and other minor dependency updates

# v1.5.0: September 27, 2024
* Add support for AMQP heartbeat/retry/retrydelay #549
* Fix pickle unmarshalling of big integers #547
* Various minor dependency updates

# v1.4.5: March 13, 2024
* Update go version to build from 1.21.8 for carbon-relay-ng images #544

# v1.4.4: July 27, 2023
* Update go images used to build carbon-relay-ng to 1.20.5 #518
* Various minor dependency updates

# v1.4.3: May 5, 2023
* Bump go mod version to 1.20 and update go images used to build carbon-relay-ng to 1.20.4 #518

# v1.4.2: April 19, 2023
* Update go images used to build carbon-relay-ng to 1.20.3 #517

# v1.4.1: March 9, 2023
* Bump `golang.org/x/crypto` to 0.1.0 #514

# v1.4.0: February 28, 2023
* Add ARM64 packages #479, #507

# v1.3.0: February 9, 2023

* retain Grafana Cloud/GEM fields when parsing storage-schemas.conf #502

# v1.2.1: patch maintenance release. December 22, 2022

* build using go 1.18 #497
* from now on embed binaries on github releases using goreleaser #498

# v1.2: minor maintenance release. March 4, 2022

* build using latest version of go, 1.17.3. #482
* tweaks to connection closing. #481
* experimental consistentHashing-v2 hashing scheme which aims to be compatible with more recent versions of carbon-relay. #477
* adopt the term "blocklist" for blocking metrics.  The existing config options "blacklist" and commands "addBlack" are replaced
  with "blocklist" and "addBlock" respectively.  The old option and command will keep working for the time being, but users are recommended
  to update their configuration because the old options will be removed at the next major release. #473
* grafanaNet route: make publishing storage-aggregation.conf optional. This is a breaking change for the `addRoute grafanaNet ...` command, as `aggregationFile=` now needs to be prepended to the aggregation file value. (Note that this command is only used by the experimental tcp admin interface, and the deprecated config init commands.) #485
* Update Jsonnet lib to work with latest Tanka version #487

# v1.1: July 7, 2021

* improve error messages, especially for storage-aggregation.conf #470
* include example storage-schemas.conf and update k8s jsonnet mixin #472
* Support storage aggregation patterns special chars #471

# v1.0: June 24, 2021

* grafanaNet route: add errBackoffMin and errBackoffFactor controls to control backoff retry timing. #465
* grafanaNet route: automatically publish stripped (from comments) storage-aggregation.conf, similar to storage-schemas.conf
  This enables an upcoming new feature which allows customizing the configs on Grafana Cloud Graphite V5. #466
* FreeBSD support. #468

# v0.14.0: May 31, 2021

Important:
Docker images are no longer being pushed to raintank/carbon-relay-ng, only to grafana/carbon-relay-ng
See https://hub.docker.com/r/grafana/carbon-relay-ng for all Docker image updates.

* Deprecate (remove) pushing to "raintank" docker hub account. #449
* Kafkamdm: make sasl mechanism configurable. #447
* Fix panels on dashboard. #387
* Fix process reporter (/proc) to behave properly in windows, freeBSD. #450
* GrafanaNet route: periodically post the schemas parsed from storage-schemas.conf (minus file comments) #458
  (this enables an upcoming new feature which allows customizing the schemas on Grafana Cloud Graphite)
* Upgrade Sarama, our kafka library from v1.19.0 to v1.23.0 #445
* add bySeriesWithTagsFnv partitioner for kafka-mdm route #460

# v0.13.0: inverted filters, kafka TLS, and more. July 10, 2020

* kafkaMdm route: allow partitionBy bySeriesWithTags. #381
* Aggregation point tracker (more detailed stats for aggregator). #382
* fix: Add error check for process reporter so we don't auto-crash for osx/windows. #390
* Add support for passing grafana.net credentials via environment variables. #393
* Add a flag to enable /debug/pprof/ on admin http port. #402
* Inverted filter criteria for matchers. #422
* Document k8s deployment. #427, #428
* substring filters have been renamed from "substr" to "sub" and "notSubstr" to "notSub" everywhere, for consistency.
  change is backwards compatible, the old "substr" parameters are still accepted in all places where they've been accepted before. #419, #423
* kafkaMdm route: TLS + SASL support. #426

# v0.12.0: better pickle support, various fixes and better aggregators. Sept 18, 2019

* support pickle protocol versions 0, 1, 2, 3 & 4 + accept pickle arrays + send pickle tuples. #341
* fix different connections to same host:port using same spool file, by adding instance to destination key. #349
* fix aggregator flush timing + limit concurrent aggregator flushing #354
* support new tsdbgw response with detailed info about invalid metrics #338
* add 'count' aggregator. #355
* Some aggregator improvements and better stats. #361, specifically:
  - assure aggregation flushing is in timestamp asc order
  - new stat for number of metrics going into aggregator: `service_is_carbon-relay-ng.instance_is_$instance.mtype_is_counter.unit_is_Metric.direction_is_in.aggregator_is_$aggregator`
  - new stat for number of metrics going out of aggregator: `service_is_carbon-relay-ng.instance_is_$instance.mtype_is_counter.unit_is_Metric.direction_is_out.aggregator_is_$aggregator`
  - new stat for number of aggregators waiting to flush: `service_is_carbon-relay-ng.instance_is_$instance.*.unit_is_aggregator.what_is_flush_waiting`
  - new stats for CPU, memory and golang GC under `carbon-relay-ng.stats.$instance.*
  - give aggregators a "key" property for use in stats (`$aggregator` above), printed at startup and in web ui.
  - process pending aggregations more eagerly - instead of waiting - when relay is somewhat loaded.
* Fix bad regex cache clean up resulting in unbounded growth #365
* Fix deadlock in keepSafe #369
* Embed bootstrap, jquery, etc, instead of pulling in from the internet. #371

# v0.11.0: memleak fix, new logging, major input refactor and more. Nov 9, 2018

* BREAKING: switch to logrus for logging. #317, #326
  If you were previously using log level notice, you should now use info.
  See [logging docs](https://github.com/grafana/carbon-relay-ng/blob/main/docs/logging.md) for more info on the log levels.
* IMPORTANT: refactor release process, docker tags and package repo. #330, #331
  see [installation docs](https://github.com/grafana/carbon-relay-ng/blob/main/docs/installation-building.md)
* fix memory leak when connections should be closed. #259, #329
* Rewrite of the inputs. pickle, tcp, udp and data reading.
  They are now plugins, handle errors better, and support timeouts.
  #286, #287, #301, #303, #306, #313, #316, #318, #320
* simplify default configs, bump default stats interval, improve config docs. #276
* Make StorageResolution for CloudWatch routes configurable. #275
* re-organize docs into separate pages. #277
* track timestamps coming in to aggregators and being too old. #280
* include sysv init file in centos6/rhel6 rpm to accommodate aws images. #283
* rewriter: support "not" clause + DRY constructor. #284
* fix connection pooling for grafanaNet route. #289
* Fix sslVerify default=true for grafanaNet route using new syntax. #291
* add support for tag appendix validation. make validation inline with grafanaNet. #292
* add request logging for admin interface. #297
* make max_procs setting optional, honor GOMAXPROCS env var. #309
* switch to circleci 2.0. #294
* switch from govendor to dep. #310

# v0.10.1: aggregator optional drop raw, grafanaNet concurrent send fix, auto-windows binaries. May 2 2018

* build windows binaries in CircleCI and save artifacts
* add support for dropping raw series consumed by an aggregator (#268, #273 )
* refactor concurrent sending for grafanaNet route. this should improve throughput and reduce buffering in some cases (#272)
* change max retry interval from 1 minute to 30 seconds, so service can restore quicker.

# v0.10.0: google pubsub, amazon cloudwatch, percentile aggregators and spool name fix (finally!). Apr 2 2018

## new routes:
* google pubsub #256
* amazon cloudwatch #261, #266

## aggregators:
* percentile aggregators #265

## other
* fix overlapping spool names / stats / log entries when you use same endpoint tcp address multiple times. #258
 **names of spool files and stats will change to include route name**
* packages for debian v9/v/10/stretch #246
* grafanaNet defaults update: timeout 5s->10s, flushMaxNum 10k->5k, concurrency 10->100 #254 , #263

# v0.9.4: Release O'Time. Dec 15, 2017

various network setting fixes  351f807168c8185a8712f7ca5bd9ea6cff020d3e , #243
make windows builds work #245
Add tmpfiles.d config for centos/7 #235
Use useradd to support multiple distros #233
support variable substitution in instance, and default to $HOST #236
fix blocklist #237
add support for specifying explicit prefixFilter and/or substringFilter in aggregations, which can help perf a lot. #239
show target address for GrafanaNet routes #244

# v0.9.3: massive aggregator improvements. Oct 19, 2017

## aggregators
* massive aggregation improvements. they now run faster (sometimes by 20x), use less memory and can handle much more load. see #227 , #230
* add derive (rate) aggregator #230
* aggregator regex cache, which lowers cpu usage and increases the max workload, in exchange for a bit more ram usage. #227 By default, the cache is enabled for aggregators set up via commands (init commands in the config) but disabled for aggregators configured via config sections (due to a limitation in our config library)

## docs
* add dashboard explanation screenshot #208

## packaging and versioning

* fix logging on cent6/amzn/el6 #224 43b265d267f27c7090f15c21ff356fee1da48734
* Fix the creation of `/var/run/carbon-relay-ng` directory #213
* `version` argument to get the version

## other
* disable http2.0 support for grafanaNet route, since an incompatibility with nginx was resulting in bogus `400 Bad Request` responses. **note** this fix does not properly work, use 0.9.3-1 instead.
* track allocated memory

# v0.9.2: non-blocking mode, better monitoring. Sep 14, 2017

* make blocking behavior configurable for kafkaMdm and grafanaNet routes.  previously, grafanaNet was blocking, kafkaMdm was non-blocking.  Now both default to non-blocking, but you can specify `blocking=true`.
  - nonblocking (default): when the route's buffer fills up, data will be discarded for that route, but everything else (e.g. other routes) will be unaffected. If you set your buffers large enough this won't be an issue.
rule of thumb: rate in metrics/s times how many seconds you want to be able to buffer in case of downstream issues. memory used will be `bufSize * 100B` or use `bufSize * 150B` to be extra safe.
  - blocking: when the route's buffer fills up, ingestion into the route will slow down/block, providing backpressure to the clients, and also blocking other routes from making progress. use this only if you know what you're doing and have smart clients that can gracefully handle the backpressure
* monitor queue drops for non-blocking queues
* [document route options better](https://github.com/grafana/carbon-relay-ng/blob/main/docs/routes.md)
* monitor queue size and ram used #218
* preliminary support for parsing out the new graphite tag format (kafkaMdm and grafanaNet route only)

the included, dashboard is updated accordingly. and also on https://grafana.com/dashboards/338

# v0.9.1: more tuneables for destinations. Aug 16, 2017

these new settings were previously hardcoded (to the values that are now the defaults):
```
connbuf=<int>                 connection buffer (how many metrics can be queued, not written into network conn). default 30k
iobuf=<int>                   buffered io connection buffer in bytes. default: 2M
spoolbuf=<int>                num of metrics to buffer across disk-write stalls. practically, tune this to number of metrics in a second. default: 10000
spoolmaxbytesperfile=<int>    max filesize for spool files. default: 200MiB (200 * 1024 * 1024)
spoolsyncevery=<int>          sync spool to disk every this many metrics. default: 10000
spoolsyncperiod=<int>         sync spool to disk every this many milliseconds. default 1000
spoolsleep=<int>              sleep this many microseconds(!) in between ingests from bulkdata/redo buffers into spool. default 500
unspoolsleep=<int>            sleep this many microseconds(!) in between reads from the spool, when replaying spooled data. default 10
```

# v0.9.0: some fixes (requires config change). Jul 6, 2017

* unrouteable messages should be debug not notice #198
* rewrite before aggregator; fix race condition, sometimes incorrect aggregations and occasional panics #199
* config parsing fix #175
* kafka-mdm: support multiple brokers. fix #195
* bugfix: make init section work again. fix #201
**attention init section must be changed** from:
```
init = ...
```
to:
```
[init]
cmds = ...
```

note : this was previously released as 0.8.9 but the breaking config change warrants a major version bump.

# v0.8.8: 3 new aggregators, better input plugins and packaging; and better config method. Jun 19, 2017

## inputs
better pickle input #174
better amqp input options #168, update amqp library 3e876644819001f0ad21ac0bc427a5eff9cb7332
kafka input logging fixes #188

## config
more proper config format so you don't have to use init commands. #183

## aggregations

add last, delta and stdev aggregator #191, #194

## packaging
Add tmpfiles.d config for deb package #179
prevent erasing of configs #181
add root certs to docker container for better grafanaCloud experience #180

# v0.8.7: better docs and stuff

minor release

# v0.8.6: multi-line amqp. Jun 19, 2017

See #165

# v0.8.5: amqp input, min/max aggregators, and more. Mar 7, 2017

* fix metrics initialisation (#150)
* update to new metrics2.0 format (mtype instead of target_type)
* refactor docker build process (#158)
* amqp input (#160)
* allow grafanaNet route to make concurrent connections (#153) to Grafana hosted metrics
* add min/max aggregators (#161)
* add kafka-mdm route for metrictank (#161)

# v0.8: Growing up a little. Dec 21, 2016

- build packages for ubuntu, debian, centos and automatically push to circleCI upon successfull builds (https://github.com/grafana/carbon-relay-ng#installation)
- add pickle input (#140)
- publish [dashboard on grafana.net](https://grafana.net/dashboards/338)
- fix build for go <1.6 (#118)
- allow overriding docker commandline (#124)
- validation: tuneable validation of metrics2.0 messages. switch default legacy validation to medium, strict was too strict for many. show validation level in UI. Add time-order validation on a per-key basis
- support rewriting of metrics
- document limitations
- grafana.net route updates and doc updates
- re-organize code into modules
- various small improvements
- remove seamless restart stuff.  not sure anymore if it still worked, especially now that we have two listeners.  we didn't maintain it well, and no-one seems to care

**known issues**:
there's some open tickets on github for various smaller issues, but one thing that has been impacting people a lot for a long time now is memory usage growing indefinitely when instrumentation is not configured (because it keeps accumulating internal metrics).
The solution is to configure `graphite_addr` to an address it can send metrics to (e.g. its own input address)
see [ticket 50](https://github.com/grafana/carbon-relay-ng/issues/50) for more info

# v0.7: 200 stars. May 23, 2016

changes:
- regex / substring / prefix support for blocklist
- support more management functionality in admin UI + show dest online status
- properly set default endpoint settings
- configurable maxprocs setting
- consistent hashing (functions same as in official carbon but without replication >1)
- pidfile support
- include example init file & dockerfile
- switch to govendor for native vendoring && update some dependencies
- UDP listener
- pull in js libraries over https
- make validation configurable
- update included dashboard for grafana3
- rate controls for profiling
- experimental new grafana.net hosted metrics route
- refactored imperatives (commands) parsing
- fix versioning when building packages

# v0.6: overhauled codebase, non-blocking operations, more solid. May 3, 2015

- lots of code refactoring.  non-blocking operations
- new config format with imperative commands
- extensive internal stats & grafana dashboard
- a more extensive routing system (which can support round robin & hashing) (see #23)
- aggregators
- use party tool for vendoring dependencies
- metric validator with json endpoint for inspection
- performance fixes

probably more. (was blocklists new?)
note: the admin interfaces need more work.

# v0.5: extended version with more features. May 3, 2015

extended version with config file, http and telnet interfaces, statsd for internal instrumentation, disk spooling support, but still blocking sends

# v0.1: initial release. May 3, 2015

initial, simple version that used commandline args to configure. no admin interfaces
basically the version written by Richard Crowley with some minor patches

note that this version uses blocking sends (1 endpoint down blocks the program)
