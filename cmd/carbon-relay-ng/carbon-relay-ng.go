// carbon-relay-ng
// route traffic to anything that speaks the Graphite Carbon protocol (text or pickle)
// such as Graphite's carbon-cache.py, influxdb, ...
package main

import (
	"flag"
	"fmt"
	"net"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/BurntSushi/toml"
	"github.com/Dieterbe/go-metrics"
	"github.com/graphite-ng/carbon-relay-ng/badmetrics"
	"github.com/graphite-ng/carbon-relay-ng/cfg"
	"github.com/graphite-ng/carbon-relay-ng/destination"
	"github.com/graphite-ng/carbon-relay-ng/imperatives"
	"github.com/graphite-ng/carbon-relay-ng/input"
	"github.com/graphite-ng/carbon-relay-ng/route"
	"github.com/graphite-ng/carbon-relay-ng/stats"
	tbl "github.com/graphite-ng/carbon-relay-ng/table"
	"github.com/graphite-ng/carbon-relay-ng/ui/telnet"
	"github.com/graphite-ng/carbon-relay-ng/ui/web"
	m20 "github.com/metrics20/go-metrics20/carbon20"
	logging "github.com/op/go-logging"

	"strconv"
	"strings"
	"time"
)

var (
	config_file      string
	config           cfg.Config
	to_dispatch      = make(chan []byte)
	table            *tbl.Table
	cpuprofile       = flag.String("cpuprofile", "", "write cpu profile to file")
	blockProfileRate = flag.Int("block-profile-rate", 0, "see https://golang.org/pkg/runtime/#SetBlockProfileRate")
	memProfileRate   = flag.Int("mem-profile-rate", 512*1024, "0 to disable. 1 for max precision (expensive!) see https://golang.org/pkg/runtime/#pkg-variables")
	badMetrics       *badmetrics.BadMetrics
)

var log = logging.MustGetLogger("carbon-relay-ng")

func init() {
	var format = "%{color}%{time:15:04:05.000000} ▶ %{level:.4s} %{color:reset} %{message}"
	logBackend := logging.NewLogBackend(os.Stderr, "", 0)
	logging.SetFormatter(logging.MustStringFormatter(format))
	logging.SetBackend(logBackend)

	input.SetLogger(log)
	tbl.SetLogger(log)
	route.SetLogger(log)
	destination.SetLogger(log)
	telnet.SetLogger(log)
	web.SetLogger(log)
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
	runtime.SetBlockProfileRate(*blockProfileRate)
	runtime.MemProfileRate = *memProfileRate

	// validation defaults
	config.Validation_level_legacy.Level = m20.MediumLegacy
	config.Validation_level_m20.Level = m20.MediumM20

	config_file = "/etc/carbon-relay-ng.ini"
	if 1 == flag.NArg() {
		config_file = flag.Arg(0)
	}

	if _, err := toml.DecodeFile(config_file, &config); err != nil {
		log.Error("Cannot use config file '%s':\n", config_file)
		log.Error(err.Error())
		usage()
		return
	}
	//runtime.SetBlockProfileRate(1) // to enable block profiling. in my experience, adds 35% overhead.

	levels := map[string]logging.Level{
		"critical": logging.CRITICAL,
		"error":    logging.ERROR,
		"warning":  logging.WARNING,
		"notice":   logging.NOTICE,
		"info":     logging.INFO,
		"debug":    logging.DEBUG,
	}
	level, ok := levels[config.Log_level]
	if !ok {
		log.Error("unrecognized log level '%s'\n", config.Log_level)
		return
	}
	logging.SetLevel(level, "carbon-relay-ng")
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if len(config.Instance) == 0 {
		log.Error("instance identifier cannot be empty")
		os.Exit(1)
	}

	runtime.GOMAXPROCS(config.Max_procs)

	log.Notice("===== carbon-relay-ng instance '%s' starting. =====\n", config.Instance)
	stats.New(config.Instance)

	if config.Pid_file != "" {
		f, err := os.Create(config.Pid_file)
		if err != nil {
			fmt.Println("error creating pidfile:", err.Error())
			os.Exit(1)
		}
		_, err = f.Write([]byte(strconv.Itoa(os.Getpid())))
		if err != nil {
			fmt.Println("error writing to pidfile:", err.Error())
			os.Exit(1)
		}
		f.Close()
	}

	input.InitMetrics()

	if config.Instrumentation.Graphite_addr != "" {
		addr, err := net.ResolveTCPAddr("tcp", config.Instrumentation.Graphite_addr)
		if err != nil {
			log.Fatal(err)
		}
		go metrics.Graphite(metrics.DefaultRegistry, time.Duration(config.Instrumentation.Graphite_interval)*time.Millisecond, "", addr)
	}

	log.Notice("creating routing table...")
	maxAge, err := time.ParseDuration(config.Bad_metrics_max_age)
	if err != nil {
		log.Error("could not parse badMetrics max age")
		log.Error(err.Error())
		os.Exit(1)
	}
	badMetrics = badmetrics.New(maxAge)
	table = tbl.New(config.Spool_dir)
	log.Notice("initializing routing table...")
	for i, cmd := range config.Init {
		log.Notice("applying: %s", cmd)
		err = imperatives.Apply(table, cmd)
		if err != nil {
			log.Error("could not apply init cmd #%d", i+1)
			log.Error(err.Error())
			os.Exit(1)
		}
	}
	tablePrinted := table.Print()
	log.Notice("===========================")
	log.Notice("========== TABLE ==========")
	log.Notice("===========================")
	for _, line := range strings.Split(tablePrinted, "\n") {
		log.Notice(line)
	}

	if config.Listen_addr != "" {
		_, err = input.NewPlain(config, config.Listen_addr, table, badMetrics)
		if err != nil {
			log.Error(err.Error())
			os.Exit(1)
		}
	}

	if config.Pickle_addr != "" {
		_, err = input.NewPickle(config, config.Pickle_addr, table, badMetrics)
		if err != nil {
			log.Error(err.Error())
			os.Exit(1)
		}
	}

	if config.Admin_addr != "" {
		go func() {
			err := telnet.Start(config.Admin_addr, table)
			if err != nil {
				fmt.Println("Error listening:", err.Error())
				os.Exit(1)
			}
		}()
	}

	if config.Http_addr != "" {
		go web.Start(config.Http_addr, config, table, badMetrics)
	}

	select {}
}
