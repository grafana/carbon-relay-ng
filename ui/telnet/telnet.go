package telnet

import (
	"errors"
	"net"
	"strings"

	"github.com/graphite-ng/carbon-relay-ng/imperatives"
	tbl "github.com/graphite-ng/carbon-relay-ng/table"
	"github.com/graphite-ng/carbon-relay-ng/telnet"
	"go.uber.org/zap"
)

var table *tbl.Table

func tcpViewHandler(req telnet.Req) (err error) {
	if len(req.Command) != 1 {
		return errors.New("extraneous arguments")
	}
	(*req.Conn).Write([]byte(table.Print() + "\n--\n"))
	return
}

func tcpModHandler(req telnet.Req) (err error) {
	err = imperatives.Apply(table, strings.Join(req.Command, " "))
	if err != nil {
		return err
	}
	(*req.Conn).Write([]byte("ok\n"))
	return
}

func tcpHelpHandler(req telnet.Req) (err error) {
	writeHelp(*req.Conn, []byte(""))
	return
}
func tcpDefaultHandler(req telnet.Req) (err error) {
	writeHelp(*req.Conn, []byte("unknown command\n"))
	return
}

func writeHelp(conn net.Conn, write_first []byte) { // bytes.Buffer
	//write_first.WriteTo(conn)
	conn.Write(write_first)
	help := `
commands:
    help                                         show this menu
    view                                         view full current routing table

    addBlack <prefix|sub|regex> <substring>      blacklist (drops matching metrics as soon as they are received)

    addRewriter <old> <new> <max>                add rewriter that will rewrite all old to new, max times
                                                 use /old/ to specify a regular expression match, with support for ${1} style identifiers in new

    addAgg <func> <match> <fmt> <interval> <wait> [cache=true/false] add a new aggregation rule.
             <func>:                             aggregation function to use
               avg
               delta
               derive
               last
               max
               min
               stdev
               sum
             <match>
               regex=<str>                       mandatory. regex to match incoming metrics. supports groups (numbered, see fmt)
               sub=<str>                         substring to match incoming metrics before matching regex (can save you CPU)
               prefix=<str>                      prefix to match incoming metrics before matching regex (can save you CPU). If not specified, will try to automatically determine from regex.
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
               sub=<str>                         only take in metrics that match this substring
               regex=<regex>                     only take in metrics that match this regex (expensive!)
             <dest>: <addr> <opts>
               <addr>                            a tcp endpoint. i.e. ip:port or hostname:port
                                                 for consistentHashing routes, an instance identifier can also be present:
                                                 hostname:port:instance
                                                 The instance is used to disambiguate multiple endpoints on the same host, as the Carbon-compatible consistent hashing algorithm does not take the port into account.
               <opts>:
                   prefix=<str>                  only take in metrics that have this prefix
                   sub=<str>                     only take in metrics that match this substring
                   regex=<regex>                 only take in metrics that match this regex (expensive!)
                   flush=<int>                   flush interval in ms
                   reconn=<int>                  reconnection interval in ms
                   pickle={true,false}           pickle output format instead of the default text protocol
                   spool={true,false}            enable spooling for this endpoint

    addDest <routeKey> <dest>                    not implemented yet

    modDest <routeKey> <dest> <opts>:            modify dest by updating one or more space separated option strings
                   addr=<addr>                   new tcp address
                   prefix=<str>                  new matcher prefix
                   sub=<str>                     new matcher substring
                   regex=<regex>                 new matcher regex

    modRoute <routeKey> <opts>:                  modify route by updating one or more space separated option strings
                   prefix=<str>                  new matcher prefix
                   sub=<str>                     new matcher substring
                   regex=<regex>                 new matcher regex

    delRoute <routeKey>                          delete given route

`
	conn.Write([]byte(help))
}

func Start(addr string, t *tbl.Table) error {
	table = t
	telnet.HandleFunc("add", tcpModHandler)
	telnet.HandleFunc("del", tcpModHandler)
	telnet.HandleFunc("mod", tcpModHandler)
	telnet.HandleFunc("view", tcpViewHandler)
	telnet.HandleFunc("help", tcpHelpHandler)
	telnet.HandleFunc("", tcpDefaultHandler)
	zap.S().Infof("admin TCP listener starting on %v", addr)
	return telnet.ListenAndServe(addr)
}
