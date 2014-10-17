carbon-relay-ng
===============

A relay for carbon streams, in go.
Like carbon-relay from the graphite project, except it:


 * performs (much) better.
 * you can adjust the routing table at runtime, in real time using the web or telnet interface.
 * can be restarted without dropping packets
 * supports a per-route spooling policy.
   (i.e. in case of an endpoint outage, we can temporarily queue the data up to disk and resume later)
 * you can choose between plaintext or pickle output, per route.
 

This makes it easy to fanout to other tools that feed in on the metrics.
Or balance/split load, or provide redundancy, or partition the data, etc.
This pattern allows alerting and event processing systems to act on the data as it is received (which is much better than repeated reading from your storage)


![screenshot](https://raw.githubusercontent.com/graphite-ng/carbon-relay-ng/master/screenshot.png)


Future work aka what's missing
------------------------------

* multi-node clustered high availability (open for discussion whether it's worth it)
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


Concepts
--------

You have 1 master routing table.  This table contains 0-N routes.  Each route can contain 0-M destinations (tcp endpoints)

First: "matching": you can match metrics on one or more of: prefix, substring, or regex.  All 3 default to "" (empty string, i.e. allow all).
The conditions are AND-ed.  Regexes are more resource intensive and hence should, and often can be avoided.

* All incoming matrics get filtered through the blacklist and then go into the table.
* The table sends the metric to any routes that matches
* The route can have different behaviors, based on its type:

  * sendAllMatch: send all metrics to all the defined endpoints (possibly, and commonly only 1 endpoint).
  * sendFirstMatch: send the metrics to the first endpoint that matches it.
  * consistent hashing: the route is a CH pool (not implemented)
  * round robin: the route is a RR pool (not implemented)



Configuration
-------------


Look at the included carbon-relay-ng.ini, it should be self describing.
In the init option you can create routes, populate the blacklist, etc using the same command as the telnet interface, detailed below.
This mechanism is choosen so we can reuse the code, instead of doing much configuration boilerplate code which would have to execute on
a declarative specification.  We can just use the same imperative commands since we just set up the initial state here.


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


