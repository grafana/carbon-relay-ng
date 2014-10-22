package main

import (
	"errors"
	"github.com/graphite-ng/carbon-relay-ng/telnet"
	"log"
	"net"
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func tcpViewHandler(req telnet.Req) (err error) {
	if len(req.Command) != 2 {
		return errors.New("extraneous arguments")
	}
	(*req.Conn).Write([]byte(table.Print() + "\n--\n"))
	return
}

func tcpModHandler(req telnet.Req) (err error) {
	err = applyCommand(table, req.Command[1])
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
    help                                     show this menu
    route list                               list routes
    route add <key> [pattern] <addr> <spool> add the route. (empty pattern allows all). (spool has to be 1 or 0)
    route del <key>                          delete the matching route
    route patt <key> [pattern]               update pattern for given route key.  (empty pattern allows all)

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
	log.Printf("admin TCP listener starting on %v", addr)
	return telnet.ListenAndServe(addr)
}
