carbon-relay-ng
===============

A relay for carbon streams, in go.
Like carbon-relay from the graphite project, except it:


 * performs (much) better.
 * you can adjust the routing table at runtime, in real time using the web or telnet interface.
 * can be restarted without dropping packets
 

This makes it easy to fanout to other tools that feed in on the metrics.
Or balance load, or provide redundancy (see "first_only" config paramater), or partition the data, etc.
This pattern allows alerting and event processing systems to act on the data as it is received (which is much better than repeated reading from your storage)


![screen shot 2014-07-24 at 7 01 13 pm](https://cloud.githubusercontent.com/assets/465717/3697144/b1efce7e-139f-11e4-83d1-c6e659fa093a.png)


Future work aka what's missing
------------------------------

* queueing/disk spooling policy to bridge remote outage (now we just drop packets to remote if it goes down)
* support for pickle protocol, if anyone cares enough to implement it (libraries for pickle in Go [already exist](https://github.com/kisielk/og-rek))
* pub-sub interface, maybe
* consistent hashing across different endpoints, if it can be implemented in an elegant way.  (note that this would still be a hack and mostly aimed for legacy setups, [decent storage has redundancy and distribution built in properly ](http://dieter.plaetinck.be/on-graphite-whisper-and-influxdb.html).


Installation
------------

    export GOPATH=/some/path/
    go get github.com/graphite-ng/carbon-relay-ng
    cd "$GOPATH/github.com/graphite-ng/carbon-relay-ng"
    go build carbon-relay-ng.go
    mv carbon-relay-ng /usr/local/bin/
    cp carbon-relay-ng.ini /etc/

Usage
-----

<pre><code>carbon-relay-ng [-cpuprofile <em>cpuprofile-file</em>] <em>config-file</em></code></pre>


Web interface
-------------

Should be self explanatory.


Admin interface
---------------

    telnet <host> <port>
    
commands:

    help                             show this menu
    route list                       list routes
    route add <key> [pattern] <addr> add the route. (empty pattern allows all)
    route del <key>                  delete the matching route
    route patt <key> [pattern]       update pattern for given route key.  (empty pattern allows all)


