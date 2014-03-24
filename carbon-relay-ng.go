// carbon-relay-ng - route traffic to Graphite's carbon-cache.py
package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/rcrowley/goagain"
	"io"
	"log"
	"net"
	"os"
	"regexp"
	"strings"
	"time"
)

type route struct {
	re *regexp.Regexp
	ch chan []byte
}

var (
	firstMatchOnly = flag.Bool(
		"f",
		false,
		"relay only to the first matching route",
	)
	listenAddr = flag.String("l", "0.0.0.0:2003", "listen address")
)

func accept(l *net.TCPListener, routes []route) {
	for {
		c, err := l.AcceptTCP()
		if nil != err {
			log.Println(err)
			break
		}
		go handle(c, routes)
	}
}

func dispatch(buf []byte, routes []route) {
	i := 0
	for _, r := range routes {
		if r.re.Match(buf) {
			i++
			r.ch <- buf
			if *firstMatchOnly {
				return
			}
		}
	}
	if 0 == i {
		log.Printf("unrouteable: %s\n", buf)
	}
}

func handle(c *net.TCPConn, routes []route) {
	defer c.Close()
	// TODO c.SetTimeout(60e9)
	r := bufio.NewReaderSize(c, 4096)
	for {
		buf, isPrefix, err := r.ReadLine()
		if nil != err {
			if io.EOF != err {
				log.Println(err)
			}
			break
		}
		if isPrefix { // TODO Recover from partial reads.
			log.Println("isPrefix: true")
			break
		}
		buf = append(buf, '\n')
		line := make([]byte, len(buf), len(buf))
		copy(line, buf)
		dispatch(line, routes)
	}
}

func init() {
	log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)
}

func main() {

	flag.Usage = usage
	flag.Parse()

	// Parse the remaining arguments as routing patterns pointing to relay
	// addresses.  The empty pattern matches everything.
	if 0 == flag.NArg() {
		log.Println("no routing patterns found")
	}
	routes := make([]route, flag.NArg())
	for i, arg := range flag.Args() {
		ss := strings.Split(arg, "=")
		if 2 != len(ss) {
			log.Printf("invalid argument %s\n", arg)
			os.Exit(1)
		}
		re, err := regexp.Compile(ss[0])
		if nil != err {
			log.Println(err)
			os.Exit(1)
		}
		ch := make(chan []byte)
		routes[i] = route{re, ch}
		raddr, err := net.ResolveTCPAddr("tcp", ss[1])
		if nil != err {
			log.Println(err)
			os.Exit(1)
		}
		go relay(ch, raddr)
	}

	// TODO HTTP management API.

	// Follow the goagain protocol, <https://github.com/rcrowley/goagain>.
	l, ppid, err := goagain.GetEnvs()
	if nil != err {
		laddr, err := net.ResolveTCPAddr("tcp", *listenAddr)
		if nil != err {
			log.Println(err)
			os.Exit(1)
		}
		log.Printf("listening on %v", laddr)
		l, err = net.ListenTCP("tcp", laddr)
		if nil != err {
			log.Println(err)
			os.Exit(1)
		}
		go accept(l.(*net.TCPListener), routes)
	} else {
		log.Printf("resuming listening on %v", l.Addr())
		go accept(l.(*net.TCPListener), routes)
		if err := goagain.KillParent(ppid); nil != err {
			log.Println(err)
			os.Exit(1)
		}
	}
	if err := goagain.AwaitSignals(l); nil != err {
		log.Println(err)
		os.Exit(1)
	}

}

func relay(ch chan []byte, raddr *net.TCPAddr) {
	var buf []byte
	laddr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0")
	for {
		c, err := net.DialTCP("tcp", laddr, raddr)
		if nil != err {
			log.Println(err)
			time.Sleep(10e9)
		}
		// TODO c.SetTimeout(60e9)
		if nil != buf && !write(buf, c) {
			continue // TODO Decide when to drop this buffer and move on.
		}
		for {
			buf := <-ch
			if !write(buf, c) {
				break
			}
		}
	}
}

func usage() {
	fmt.Fprintln(
		os.Stderr,
		"Usage: carbon-relay-ng [-f] [-l [<ip>]:<port>] [<pattern>]=[<ip>]:<port>[...]",
	)
	flag.PrintDefaults()
}

func write(buf []byte, c *net.TCPConn) bool {
	n, err := c.Write(buf)
	if nil != err {
		log.Println(err)
		c.Close()
		return false
	}
	if len(buf) != n {
		log.Printf("truncated: %s\n", buf)
		c.Close()
		return false
	}
	return true
}
