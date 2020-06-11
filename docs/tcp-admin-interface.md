# TCP Interface

Admin commands that you can execute on a live carbon-relay-ng daemon.
Note: you can also have a relay execute these commands at bootup via the init.cmds setting


commands:

    help                                         show this menu
    view                                         view full current routing table

    addBlack <prefix|sub|regex> <substring>      blacklist (drops matching metrics as soon as they are received)

    addRewriter <old> <new> <max>                add rewriter that will rewrite all old to new, max times
                                                 use /old/ to specify a regular expression match, with support for ${1} style identifiers in new

    addAgg <func> <match> <fmt> <interval> <wait> [cache=true/false] add a new aggregation rule.
             <func>:                             aggregation function to use
               avg
               count
               delta
               derive
               last
               max
               min
               stdev
               sum
             <match>
               regex=<str>                       mandatory. regex to match incoming metrics. supports groups (numbered, see fmt)
               notRegex=<str>                    regex to check against incoming metrics, inverted (only metrics where the regex doesn't match pass)
               sub=<str>                         substring to match incoming metrics before matching regex (can save you CPU)
               notSub=<str>                      inverted substring filter, metrics which do not contain this string pass the filter
               prefix=<str>                      prefix to match incoming metrics before matching regex (can save you CPU). If not specified, will try to automatically determine from regex.
               notPrefix=<str>                   inverted prefix filter, metrics which do not start with this string pass the filter
             <fmt>                               format of output metric. you can use $1, $2, etc to refer to numbered groups
             <interval>                          align odd timestamps of metrics into buckets by this interval in seconds.
             <wait>                              amount of seconds to wait for "late" metric messages before computing and flushing final result.


    addRoute <type> <key> [opts]   <dest>  [<dest>[...]] add a new route. note 2 spaces to separate destinations
             <type>:
               sendAllMatch                      send metrics in the route to all destinations
               sendFirstMatch                    send metrics in the route to the first one that matches it
               consistentHashing                 distribute metrics between destinations using a hash algorithm
             <opts>:
               prefix=<str>                      only take in metrics that have this prefix
               notPrefix=<str>                   only take in metrics that don't have this prefix
               sub=<str>                         only take in metrics that match this substring
               notSub=<str>                      only take in metrics that don't match this substring
               regex=<regex>                     only take in metrics that match this regex (expensive!)
               notRegex=<regex>                  only take in metrics that don't match this regex (expensive!)
             <dest>: <addr> <opts>
               <addr>                            a tcp endpoint. i.e. ip:port or hostname:port
                                                 for consistentHashing routes, an instance identifier can also be present:
                                                 hostname:port:instance
                                                 The instance is used to disambiguate multiple endpoints on the same host, as the Carbon-compatible consistent hashing algorithm does not take the port into account.
               <opts>:
                   prefix=<str>                  only take in metrics that have this prefix
                   notPrefix=<str>               only take in metrics that don't have this prefix
                   sub=<str>                     only take in metrics that match this substring
                   notSub=<str>                  only take in metrics that don't match this substring
                   regex=<regex>                 only take in metrics that match this regex (expensive!)
                   notRegex=<regex>              only take in metrics that don't match this regex (expensive!)
                   flush=<int>                   flush interval in ms
                   reconn=<int>                  reconnection interval in ms
                   pickle={true,false}           pickle output format instead of the default text protocol
                   spool={true,false}            enable spooling for this endpoint
                   connbuf=<int>                 connection buffer (how many metrics can be queued, not written into network conn). default 30k
                   iobuf=<int>                   buffered io connection buffer in bytes. default: 2M
                   spoolbuf=<int>                num of metrics to buffer across disk-write stalls. practically, tune this to number of metrics in a second. default: 10000
                   spoolmaxbytesperfile=<int>    max filesize for spool files. default: 200MiB (200 * 1024 * 1024)
                   spoolsyncevery=<int>          sync spool to disk every this many metrics. default: 10000
                   spoolsyncperiod=<int>         sync spool to disk every this many milliseconds. default 1000
                   spoolsleep=<int>              sleep this many microseconds(!) in between ingests from bulkdata/redo buffers into spool. default 500
                   unspoolsleep=<int>            sleep this many microseconds(!) in between reads from the spool, when replaying spooled data. default 10



    addDest <routeKey> <dest>                    not implemented yet

    modDest <routeKey> <dest> <opts>:            modify dest by updating one or more space separated option strings
                   addr=<addr>                   new tcp address
                   prefix=<str>                  new matcher prefix
                   notPrefix=<str>               new matcher not prefix
                   sub=<str>                     new matcher substring
                   notSub=<str>                  new matcher not substring
                   regex=<regex>                 new matcher regex
                   notRegex=<regex>              new matcher not regex

    modRoute <routeKey> <opts>:                  modify route by updating one or more space separated option strings
                   prefix=<str>                  new matcher prefix
                   notPrefix=<str>               new matcher not prefix
                   sub=<str>                     new matcher substring
                   notSub=<str>                  new matcher not substring
                   regex=<regex>                 new matcher regex
                   notRegex=<regex>              new matcher not regex

    delRoute <routeKey>                          delete given route
