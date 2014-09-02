carbon-relay-ng
===============

A relay for carbon streams, in go.
Like carbon-relay from the graphite project, except it:


 * performs (much) better.
 * you can adjust the routing table at runtime, in real time using the web or telnet interface.
 * can be restarted without dropping packets
 * supports a per-route spooling policy.
   (i.e. in case of an endpoint outage, we can temporarily queue the data up to disk and resume later)
 

This makes it easy to fanout to other tools that feed in on the metrics.
Or balance load, or provide redundancy (see "first_only" config paramater), or partition the data, etc.
This pattern allows alerting and event processing systems to act on the data as it is received (which is much better than repeated reading from your storage)


![screen shot 2014-07-24 at 7 01 13 pm](https://cloud.githubusercontent.com/assets/465717/3697144/b1efce7e-139f-11e4-83d1-c6e659fa093a.png)


Future work aka what's missing
------------------------------

* support for pickle protocol, if anyone cares enough to implement it (libraries for pickle in Go [already exist](https://github.com/kisielk/og-rek))
* pub-sub interface, maybe
* consistent hashing across different endpoints, if it can be implemented in an elegant way.  (note that this would still be a hack and mostly aimed for legacy setups, [decent storage has redundancy and distribution built in properly ](http://dieter.plaetinck.be/on-graphite-whisper-and-influxdb.html).


Building
--------

    export GOPATH=/some/path/
    export PATH="$PATH:$GOPATH/bin"
    go get -d github.com/graphite-ng/carbon-relay-ng
    go get github.com/jteeuwen/go-bindata/...
    cd "$GOPATH/src/github.com/graphite-ng/carbon-relay-ng"
    ./make.sh


Installation
------------

    You only need the compiled binary and a config.  Put them whereever you want.

Usage
-----

<pre><code>carbon-relay-ng [-cpuprofile <em>cpuprofile-file</em>] <em>config-file</em></code></pre>


Web interface
-------------

Allows you to inspect and change routing table.
(except for spooling settings).
Also you can't adjust global configuration this way.


TCP interface
-------------

Allows you to inspect and change routing table.
(except for spooling settings and remote addr).
Also you can't adjust global configuration this way.


    telnet <host> <port>
    
commands:

    help                             show this menu
    route list                       list routes
    route add <key> [pattern] <addr> add the route. (empty pattern allows all)
    route del <key>                  delete the matching route
    route patt <key> [pattern]       update pattern for given route key.  (empty pattern allows all)


