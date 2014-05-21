carbon-relay-ng
===============

What?
-----

A relay for carbon streams, in go.
Like carbon-relay from the graphite project, except it performs (much) better.
Consistent hashing is not supported, [http://dieter.plaetinck.be/on-graphite-whisper-and-influxdb.html](because you should use proper storage).
But you get a telnet admin interface over which you can adjust the routing table.
I.e.: you can modify routes at runtime, this makes it easy to fanout to other tools that feed in on the metrics, at runtime.
Or balance load, or redundancy (see "first_only" config paramater), or partition the data, etc.
Repeatedly reading the most recent data points in a Whisper file is silly.  This pattern allows alerting and event processing systems to act on the data as it is received.



Future ideas
------------
* make flexible routing available as pub-sub
* queueing/disk spooling policy to bridge remote outage (now we just drop packets to remote if it goes down)


Installation
------------

    go get github.com/graphite-ng/carbon-relay-ng

Usage
-----

<pre><code>carbon-relay-ng [-cpuprofile <em>cpuprofile-file</em>] <em>config-file</em></code></pre>

