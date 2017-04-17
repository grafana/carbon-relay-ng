The GrafanaNet route is a special route within carbon-relay-ng.

It converts graphite (carbon) input into metrics2.0 form and submits it to a grafanaCloud metrics store.
**Note: the hosted metrics store is currently not yet a public service.  Get in touch at hello@grafana.com if interested.**

# The quick way

To get started is just take the sample config, uncomment the grafanaNet line, fill in your baseUrl, key and path to storage-schemas.conf
and start the relay. That's it!

# The more detailed way

note:
* it requires a grafana.com api key and a url to the hosted store
* api key should have editor or admin role. (viewer works for now, but will be blocked in the future)
* it needs to read your graphite storage-schemas.conf to determine which intervals to use
* but will send your metrics the way you send them.
  (if you send at odd intervals, or an interval that doesn't match your storage-schemas.conf, that's how we will store it. so make sure you have this correct)
* any metric messages that don't validate are filtered out. see the admin ui to troubleshoot if needed.

## syntax

notes:
* the double space to separate the route definiton from the endpoint properties
* by specifying a prefix, sub or regex you can only send a subset of your metrics to grafana.com hosted metrics

```
addRoute grafanaNet key [prefix/sub/regex]  addr apiKey schemasFile [spool=true/false sslverify=true/false bufSize=int flushMaxNum=int flushMaxWait=int timeout=int]")
```


### matching options

substring match: `addRoute grafanaNet grafanaNet sub=<substring here>  addr...`
regex match: `addRoute grafanaNet grafanaNet regex=your-regex-here  addr...`
prefix match: `addRoute grafanaNet grafanaNet prefix=prefix-match-here  addr...`

the options can also be combined (space separated). note two spaces before the address!

note that the matching is applied to the entire metric line (including key, value and timestamp)


### other options

other options can appear after the schemasFile, space-separated.

* for schemasFile, see [storage-schemas.conf documentation](http://graphite.readthedocs.io/en/latest/config-carbon.html#storage-schemas-conf)
* bufSize: how many metrics we can queue up in the route before providing backpressure (default 1e7 i.e. 10 million)
* flushMaxNum: after this many metrics have queued up, trigger a flush (default 10k)
* flushMaxWait: after this many milliseconds, trigger a flush (default 500)
* timeout: after how many milliseconds to consider a request to the hosted metrics to timeout, so that it will retry later (default 5000)
* sslverify: disables ssl verifications, useful for test/POC setups with self signed ssl certificates (default true)
* concurrency: how many independent workers pushing data to grafanacloud (default 10)

