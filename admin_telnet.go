package main

import (
	"errors"
	"github.com/graphite-ng/carbon-relay-ng/telnet"
	"net"
	"strings"
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func tcpViewHandler(req telnet.Req) (err error) {
	if len(req.Command) != 1 {
		return errors.New("extraneous arguments")
	}
	(*req.Conn).Write([]byte(table.Print() + "\n--\n"))
	return
}

func tcpModHandler(req telnet.Req) (err error) {
	err = applyCommand(table, strings.Join(req.Command, " "))
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
    addBlack <substring>                         blacklist (drops the metric matching this as soon as it is received)
    addRoute <type> <key> [opts]   <dest>  [<dest>[...]] add a new route. note 2 spaces to separate destinations
             <type>:
               sendAllMatch                      send metrics in the route to all destinations
               sendFirstMatch                    send metrics in the route to the first one that matches it
             <opts>:
               prefix=<str>                      only take in metrics that have this prefix
               sub=<str>                         only take in metrics that match this substring
               regex=<regex>                     only take in metrics that match this regex (expensive!)
             <dest>: <addr> <opts>
               <addr>                            a tcp endpoint. i.e. ip:port or hostname:port
               <opts>:
                   prefix=<str>                  only take in metrics that have this prefix
                   sub=<str>                     only take in metrics that match this substring
                   regex=<regex>                 only take in metrics that match this regex (expensive!)
                   flush=<int>                   flush interval in ms
                   reconn=<int>                  reconnection interval in ms
                   pickle={true,false}           pickle output format instead of the default text protocol
                   spool={true,false}            enable spooling for this endpoint
    addDest <routeKey> <dest>                    not implemented yet
    modDest <routeKey> <dest> <opts>:            modify route by updating one or more space separated option strings
                   addr=<addr>                   new tcp address
                   prefix=<str>                  new matcher prefix
                   sub=<str>                     new matcher substring
                   regex=<regex>                 new matcher regex


`
	conn.Write([]byte(help))
}

func adminListener(addr string) error {
	telnet.HandleFunc("add", tcpModHandler)
	telnet.HandleFunc("del", tcpModHandler)
	telnet.HandleFunc("mod", tcpModHandler)
	telnet.HandleFunc("view", tcpViewHandler)
	telnet.HandleFunc("help", tcpHelpHandler)
	telnet.HandleFunc("", tcpDefaultHandler)
	log.Notice("admin TCP listener starting on %v", addr)
	return telnet.ListenAndServe(addr)
}
