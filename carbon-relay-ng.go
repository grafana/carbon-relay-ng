// carbon-relay-ng
// route traffic to anything that speaks the Graphite Carbon protocol,
// such as Graphite's carbon-cache.py, influxdb, ...
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/rcrowley/goagain"
	"io"
	"log"
	"net"
	"os"
	"regexp"
	"routes"
	"strings"
	"time"
)

type routeReq struct {
	Command []string
	Conn    *net.Conn // user api connection
}

type Config struct {
	Listen_addr string
	Admin_addr  string
	First_only  bool
	Routes      map[string]*Route
}

var (
	config_file   string
	config        Config
	routeRequests = make(chan routeReq)
	to_dispatch   = make(chan []byte)
)

func accept(l *net.TCPListener, config Config) {
	for {
		c, err := l.AcceptTCP()
		if nil != err {
			log.Println(err)
			break
		}
		go handle(c, config)
	}
}

func handle(c *net.TCPConn, config Config) {
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
		buf_copy := make([]byte, len(buf), len(buf))
		copy(buf_copy, buf)
		to_dispatch <- buf_copy
	}
}

func RoutingManager() {
	for {
		select {
		case buf := <-to_dispatch:
			// route according to current rules
			routed := false
			for _, r := range config.Routes {
				if r.Reg.Match(buf) {
					routed = true
					r.ch <- buf
					if config.First_only {
						break
					}
				}
			}
			if !routed {
				log.Printf("unrouteable: %s\n", buf)
			}
		case req := <-routeRequests:
			args_needed := map[string]uint{"add": 3, "del": 1, "patt", 2}
			cmd := req.Command[0]
			num_args_needed, found := args_needed[cmd]
			if !found {
				WriteHelp([]byte("unrecognized command\n"))
			}
			if len(req.Command) != num_args_needed+2 {
				WriteHelp([]byte("invalid request\n"))
				go handleApiRequest(*req.Conn)
			}
			switch cmd {
			case "add":
				key := req.Command[1]
				patt := req.Command[2]
				addr := req.Command[3]
				route := NewRoute(patt, addr)
				routes.Map[key] = route
			case "del":
				key := req.Command[1]
				_, found := routes.Map[key]
				if !found {
					go handleApiRequest(*req.Conn, []byte("unknown route"))
				}
				delete(routes.Map, key)
			case "patt":
				key := req.Command[1]
				patt := req.Command[2]
				route, found := routes.Map[key]
				if !found {
					go handleApiRequest(*req.Conn, []byte("unknown route"))
				}
				old_patt = route.Patt
				route.Patt = patt
				err := route.Compile()
				if err {
					route.Patt = old_patt
					route.Compile()
					go handleApiRequest(*req.Conn, []byte(err))
				}
			}
		}
	}
}

func init() {
	log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)
}

func writeHelp(conn net.Conn, write_first bytes.Buffer) {
	write_first.WriteTo(conn)
	help := `
commands:
    help                             show this menu
    route add <key> <pattern> <addr> add the route
    route del <key>                  delete the matching route
    route patt <key> <pattern>       update pattern for given route key

`
	conn.Write([]byte(help))
}

// handleApiRequest handles one or more api requests over the admin interface, to the extent it can.
// some operations need to be performed by the RoutingManager, so we write the request into a channel along with
// the connection.  the monitor will handle the request when it gets to it, and invoke this function again
// so we can resume handling a request.
func handleApiRequest(conn net.Conn, write_first bytes.Buffer) {
	write_first.WriteTo(conn)
	// Make a buffer to hold incoming data.
	buf := make([]byte, 1024)
	// Read the incoming connection into the buffer.
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				fmt.Println("read eof. closing")
			} else {
				fmt.Println("Error reading:", err.Error())
			}
			conn.Close()
			break
		}
		clean_cmd := strings.TrimSpace(string(buf[:n]))
		command := strings.Split(clean_cmd, " ")
		log.Println("received command: '" + clean_cmd + "'")
		switch command[0] {
		case "route":
			routeRequests <- routeReq{command[1:], &conn}
			return
		case "help":
			writeHelp(conn)
			continue
		default:
			writeHelp(conn, []byte("unknown command\n"))
		}
	}
}

func adminListener() {
	l, err := net.Listen("tcp", config.Admin_addr)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	defer l.Close()
	fmt.Println("Listening on " + config.Admin_addr)
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		go handleApiRequest(conn, bytes.Buffer{})
	}
}

func usage() {
	fmt.Fprintln(
		os.Stderr,
		"Usage: carbon-relay-ng <path-to-config>",
	)
	flag.PrintDefaults()
}

func main() {

	flag.Usage = usage
	flag.Parse()

	config_file = "/etc/carbon-relay-ng.ini"
	if 1 == flag.NArg() {
		config_file = flag.Arg(0)
	}

	if _, err := toml.DecodeFile(config_file, &config); err != nil {
		fmt.Printf("Cannot use config file '%s':\n", config_file)
		fmt.Println(err)
		return
	}
	err = routes.Init(config.Routes)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// Follow the goagain protocol, <https://github.com/rcrowley/goagain>.
	l, ppid, err := goagain.GetEnvs()
	if nil != err {
		laddr, err := net.ResolveTCPAddr("tcp", config.Listen_addr)
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
		go accept(l.(*net.TCPListener), config)
	} else {
		log.Printf("resuming listening on %v", l.Addr())
		go accept(l.(*net.TCPListener), config)
		if err := goagain.KillParent(ppid); nil != err {
			log.Println(err)
			os.Exit(1)
		}
	}
	if err := goagain.AwaitSignals(l); nil != err {
		log.Println(err)
		os.Exit(1)
	}

	go adminListener()
	go RoutingManager()

}
