package main

import (
	"errors"
	"fmt"
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
	// TODO also print route type
	// we want to print things concisely (but no smaller than the defaults below)
	// so we have to figure out the max lengths of everything first
	// the default values can be arbitrary (bot not smaller than the column titles),
	// i figured multiples of 4 should look good
	// 'R' stands for Route, 'D' for dest
	maxRKey := 8
	maxRPrefix := 4
	maxRSub := 4
	maxRRegex := 4
	maxDPrefix := 4
	maxDSub := 4
	maxDRegex := 4
	maxDAddr := 16
	maxDSpoolDir := 16

	t := table.Snapshot()
	for _, route := range t.routes {
		maxRKey = max(maxRKey, len(route.Key))
		maxRPrefix = max(maxRPrefix, len(route.Matcher.Prefix))
		maxRSub = max(maxRSub, len(route.Matcher.Sub))
		maxRRegex = max(maxRRegex, len(route.Matcher.Regex))
		for _, dest := range route.Dests {
			maxDPrefix = max(maxDPrefix, len(dest.Matcher.Prefix))
			maxDSub = max(maxDSub, len(dest.Matcher.Sub))
			maxDRegex = max(maxDRegex, len(dest.Matcher.Regex))
			maxDAddr = max(maxDAddr, len(dest.Addr))
			maxDSpoolDir = max(maxDSpoolDir, len(dest.spoolDir))
		}
	}
	routeFmt := fmt.Sprintf("%%%ds %%%ds %%%ds %%%ds\n", maxRKey+1, maxRPrefix+1, maxRSub+1, maxRRegex+1)
	destFmt := fmt.Sprintf("    %%%ds %%%ds %%%ds %%%ds %%%ds %%6s %%6s %%6s\n", maxRPrefix+1, maxRSub+1, maxRRegex+1, maxDAddr+1, maxDSpoolDir+1)
	(*req.Conn).Write([]byte(fmt.Sprintf(routeFmt, "key", "prefix", "substr", "regex")))
	for _, route := range t.routes {
		m := route.Matcher
		(*req.Conn).Write([]byte(fmt.Sprintf(routeFmt, route.Key, m.Prefix, m.Sub, m.Regex)))
		(*req.Conn).Write([]byte(fmt.Sprintf(destFmt, "prefix", "substr", "regex", "addr", "spoolDir", "spool", "pickle", "online")))
		for _, dest := range route.Dests {
			m := dest.Matcher
			(*req.Conn).Write([]byte(fmt.Sprintf(destFmt, m.Prefix, m.Sub, m.Regex, dest.Addr, dest.spoolDir, dest.Spool, dest.Pickle, dest.Online)))
		}
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
