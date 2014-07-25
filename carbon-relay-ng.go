// carbon-relay-ng
// route traffic to anything that speaks the Graphite Carbon protocol,
// such as Graphite's carbon-cache.py, influxdb, ...
package main

import (
	"./routing"
	"bufio"
	"errors"
	"flag"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/Dieterbe/statsd-go"
	"github.com/rcrowley/goagain"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"runtime/pprof"
	"strings"
)

type routeReq struct {
	Command []string
	Conn    *net.Conn // user api connection
}

type StatsdConfig struct {
	Enabled  bool
	Instance string
	Host     string
	Port     int
}

type Config struct {
	Listen_addr string
	Admin_addr  string
	Http_addr   string
	First_only  bool
	Routes      map[string]*routing.Route
	Statsd      StatsdConfig
}

var (
	config_file   string
	config        Config
	routeRequests = make(chan routeReq)
	to_dispatch   = make(chan []byte)
	routes        routing.Routes
	statsdClient  statsd.Client
	cpuprofile    = flag.String("cpuprofile", "", "write cpu profile to file")
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
		statsdClient.Increment("target_type=count.unit=Metric.direction=in")
		to_dispatch <- buf_copy
	}
}

type handleFunc struct {
	Match string
	Func  func(req routeReq) (err error)
}

var routeHandlers = make([]handleFunc, 0, 0)

func init() {
	routeHandlers = append(routeHandlers, handleFunc{
		"list",
		func(req routeReq) (err error) {
			if len(req.Command) != 1 {
				return errors.New("extraneous arguments")
			}
			longest_key := 9
			longest_patt := 9
			longest_addr := 9
			for key, route := range routes.Map {
				if len(key) > longest_key {
					longest_key = len(key)
				}
				if len(route.Patt) > longest_patt {
					longest_patt = len(route.Patt)
				}
				if len(route.Addr) > longest_addr {
					longest_addr = len(route.Addr)
				}
			}
			fmt_str := fmt.Sprintf("%%%ds %%%ds %%%ds\n", longest_key+1, longest_patt+1, longest_addr+1)
			(*req.Conn).Write([]byte(fmt.Sprintf(fmt_str, "key", "pattern", "addr")))
			for key, route := range routes.Map {
				(*req.Conn).Write([]byte(fmt.Sprintf(fmt_str, key, route.Patt, route.Addr)))
			}
			go handleApiRequest(*req.Conn, []byte("--\n"))
			return
		}})
	routeHandlers = append(routeHandlers, handleFunc{
		"add",
		func(req routeReq) (err error) {
			key := req.Command[1]
			var patt, addr string
			if len(req.Command) == 3 {
				patt = ""
				addr = req.Command[2]
			} else if len(req.Command) == 4 {
				patt = req.Command[2]
				addr = req.Command[3]
			} else {
				return errors.New("bad number of arguments")
			}

			_, found := routes.Map[key]
			if found {
				return errors.New("route with given key already exists")
			}
			route := routing.NewRoute(key, patt, addr, &statsdClient)
			err = route.Compile()
			if err != nil {
				return err
			}
			err = route.Run()
			if err != nil {
				return err
			}
			routes.Map[key] = route
			go handleApiRequest(*req.Conn, []byte("added\n"))
			return
		}})
	routeHandlers = append(routeHandlers, handleFunc{
		"del",
		func(req routeReq) (err error) {
			if len(req.Command) != 2 {
				return errors.New("bad number of arguments")
			}
			key := req.Command[1]
			route, found := routes.Map[key]
			if !found {
				return errors.New("unknown route '" + key + "'")
			}
			delete(routes.Map, key)
			err = route.Shutdown()
			if err != nil {
				// route removed from routing table but still trying to connect
				// it won't get new stuff on its input though
				return
			}
			go handleApiRequest(*req.Conn, []byte("deleted\n"))
			return
		}})
	routeHandlers = append(routeHandlers, handleFunc{
		"patt",
		func(req routeReq) (err error) {
			key := req.Command[1]
			var patt string
			if len(req.Command) == 3 {
				patt = req.Command[2]
			} else if len(req.Command) == 2 {
				patt = ""
			} else {
				return errors.New("bad number of arguments")
			}
			route, found := routes.Map[key]
			if !found {
				return errors.New("unknown route '" + key + "'")
			}
			old_patt := route.Patt
			route.Patt = patt
			err = route.Compile()
			if err != nil {
				route.Patt = old_patt
				route.Compile()
				return err
			}
			go handleApiRequest(*req.Conn, []byte("updated\n"))
			return
		}})
}

func getHandler(req routeReq) (fn *handleFunc) {
	cmd_str := strings.Join(req.Command, " ")
	for _, handler := range routeHandlers {
		if strings.HasPrefix(cmd_str, handler.Match) {
			return &handler
		}
	}
	return
}

func RoutingManager() {
	for {
		select {
		case buf := <-to_dispatch:
			routed := routes.Dispatch(buf, config.First_only)
			if !routed {
				log.Printf("unrouteable: %s\n", buf)
			}
		case req := <-routeRequests:
			if len(req.Command) == 0 {
				writeHelp(*req.Conn, []byte("invalid request\n"))
				go handleApiRequest(*req.Conn, []byte(""))
				break
			}
			fn := getHandler(req)
			if fn != nil {
				err := fn.Func(req)
				if err != nil {
					go handleApiRequest(*req.Conn, []byte(err.Error()+"\n"))
				} // if no error, func should have called handleApiRequest itself
			} else {
				writeHelp(*req.Conn, []byte("unrecognized command\n"))
				go handleApiRequest(*req.Conn, []byte(""))
			}
		}
	}
}

func init() {
	log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)
}

func writeHelp(conn net.Conn, write_first []byte) { // bytes.Buffer
	//write_first.WriteTo(conn)
	conn.Write(write_first)
	help := `
commands:
    help                             show this menu
    route list                       list routes
    route add <key> [pattern] <addr> add the route. (empty pattern allows all)
    route del <key>                  delete the matching route
    route patt <key> [pattern]       update pattern for given route key.  (empty pattern allows all)

`
	conn.Write([]byte(help))
}

// handleApiRequest handles one or more api requests over the admin interface, to the extent it can.
// some operations need to be performed by the RoutingManager, so we write the request into a channel along with
// the connection.  the monitor will handle the request when it gets to it, and invoke this function again
// so we can resume handling a request.
func handleApiRequest(conn net.Conn, write_first []byte) { // bytes.Buffer
	// write_first.WriteTo(conn)
	conn.Write(write_first)
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
			writeHelp(conn, []byte(""))
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
	log.Printf("listening on %v", config.Admin_addr)
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		go handleApiRequest(conn, []byte("")) //bytes.Buffer{})
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request, title string) {
	tc := make(map[string]interface{})
	tc["Title"] = title
	tc["routes"] = routes.Map

	templates := template.Must(template.ParseFiles("templates/base.html", "templates/index.html"))
	if err := templates.Execute(w, tc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	key := r.URL.Path[len("/edit/"):]
	route := routes.Map[key]
	fmt.Printf("Editting %s with %s - %s \n", route.Key, route.Patt, route.Addr)

	tc := make(map[string]interface{})
	tc["Title"] = title
	tc["Key"] = route.Key
	tc["Addr"] = route.Addr
	tc["Patt"] = route.Patt

	templates := template.Must(template.ParseFiles("templates/base.html", "templates/edit.html"))

	if err := templates.Execute(w, tc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	key := r.FormValue("key")
	patt := r.FormValue("patt")
	addr := r.FormValue("addr")

	_, found := routes.Map[key]
	if found {
		route := routes.Map[key]
		old_patt := route.Patt
		old_addr := route.Addr
		route.Patt = patt
		route.Addr = addr

		fmt.Printf("route patt %s %s %s \n", route.Key, route.Patt, route.Addr)

		err := route.Compile()
		if err != nil {
			route.Patt = old_patt
			route.Addr = old_addr
			route.Compile()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		route := routing.NewRoute(key, patt, addr, &statsdClient)
		fmt.Printf("route add %s %s %s \n", key, patt, addr)
		err := route.Compile()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = route.Run()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		routes.Map[key] = route
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func deleteHandler(w http.ResponseWriter, r *http.Request, title string) {
	key := r.URL.Path[len("/delete/"):]
	route, found := routes.Map[key]
	if !found {
		return
	}
	delete(routes.Map, key)
	err := route.Shutdown()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		validPath := regexp.MustCompile("^/(edit|save|delete)?(.*)$")
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func httpListener() {
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	http.HandleFunc("/delete/", makeHandler(deleteHandler))
	http.HandleFunc("/", makeHandler(homeHandler))
	http.ListenAndServe(config.Http_addr, nil)
	log.Printf("listening on %v", config.Http_addr)
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
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	log.Println("initializing routes...")
	routes = routing.NewRoutes(config.Routes, &statsdClient)
	err := routes.Init()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	statsdPrefix := fmt.Sprintf("service=carbon-relay-ng.instance=%s.", config.Statsd.Instance)
	statsdClient = *statsd.NewClient(config.Statsd.Enabled, config.Statsd.Host, config.Statsd.Port, statsdPrefix)

	// Follow the goagain protocol, <https://github.com/rcrowley/goagain>.
	l, ppid, err := goagain.GetEnvs()
	if nil != err {
		laddr, err := net.ResolveTCPAddr("tcp", config.Listen_addr)
		if nil != err {
			log.Println(err)
			os.Exit(1)
		}
		l, err = net.ListenTCP("tcp", laddr)
		if nil != err {
			log.Println(err)
			os.Exit(1)
		}
		log.Printf("listening on %v", laddr)
		go accept(l.(*net.TCPListener), config)
	} else {
		log.Printf("resuming listening on %v", l.Addr())
		go accept(l.(*net.TCPListener), config)
		if err := goagain.KillParent(ppid); nil != err {
			log.Println(err)
			os.Exit(1)
		}
	}

	if config.Admin_addr != "" {
		go adminListener()
	}

	if config.Http_addr != "" {
		go httpListener()
	}

	go RoutingManager()

	if err := goagain.AwaitSignals(l); nil != err {
		log.Println(err)
		os.Exit(1)
	}
}
