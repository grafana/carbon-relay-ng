package main

import (
	"errors"
	"fmt"
	"github.com/graphite-ng/carbon-relay-ng/telnet"
	"log"
	"net"
)

func tcpViewHandler(req telnet.Req) (err error) {
	if len(req.Command) != 2 {
		return errors.New("extraneous arguments")
	}
	longest_key := 9
	longest_patt := 9
	longest_addr := 9
	t := table.Snapshot()
	for i, route := range t.routes {
		if len(route.Key) > longest_key {
			longest_key = len(route.key)
		}
		if len(route.Patt) > longest_patt {
			longest_patt = len(route.Patt)
		}
		if len(route.Addr) > longest_addr {
			longest_addr = len(route.Addr)
		}
	}
	fmt_str := fmt.Sprintf("%%%ds %%%ds %%%ds %%8v\n", longest_key+1, longest_patt+1, longest_addr+1)
	(*req.Conn).Write([]byte(fmt.Sprintf(fmt_str, "key", "pattern", "addr", "spool")))
	for key, route := range list {
		(*req.Conn).Write([]byte(fmt.Sprintf(fmt_str, key, route.Patt, route.Addr, route.Spool)))
	}
	(*req.Conn).Write([]byte("--\n"))
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
