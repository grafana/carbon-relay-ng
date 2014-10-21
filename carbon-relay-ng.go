// carbon-relay-ng
// route traffic to anything that speaks the Graphite Carbon protocol (text or pickle)
// such as Graphite's carbon-cache.py, influxdb, ...
package main

import (
	"bufio"
	"expvar"
	"flag"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/rcrowley/goagain"
	"io"
	"log"
	"net"
	"os"
	"runtime/pprof"
)

type Config struct {
	Listen_addr string
	Admin_addr  string
	Http_addr   string
	Spool_dir   string
	First_only  bool
	Routes      []*Route
	Init        []string
	Instance    string
}

var (
	config_file string
	config      Config
	to_dispatch = make(chan []byte)
	table       *Table
	cpuprofile  = flag.String("cpuprofile", "", "write cpu profile to file")
	numIn       = Int("target_type=count.unit=Metric.direction=in")
)

func init() {
	log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)
}

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
		numIn.Add(1)
		// table should handle this as fast as it can
		table.Dispatch(buf_copy)
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
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if len(config.Instance) == 0 {
		log.Println("instance identifier cannot be empty")
		os.Exit(1)
	}
	log.Printf("===== carbon-relay-ng instance '%s' starting. =====\n", config.Instance)

	log.Println("creating routing table...")
	table = NewTable(config.Spool_dir)
	log.Println("initializing routing table...")
	var err error
	for i, cmd := range config.Init {
		fmt.Println("applying", cmd)
		err = applyCommand(table, cmd, config)
		if err != nil {
			log.Println("could not apply init cmd #", i+1)
			log.Println(err)
			os.Exit(1)
		}
	}
	fmt.Println(table.Print())
	err = table.Run()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	instance := expvar.NewString("instance")
	service := expvar.NewString("service")
	instance.Set(config.Instance)
	service.Set("carbon-relay-ng")

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
		go func() {
			err := adminListener(config.Admin_addr)
			if err != nil {
				fmt.Println("Error listening:", err.Error())
				os.Exit(1)
			}
		}()
	}

	if config.Http_addr != "" {
		go HttpListener(config.Http_addr, table)
	}

	if err := goagain.AwaitSignals(l); nil != err {
		log.Println(err)
		os.Exit(1)
	}
}
