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
	m20 "github.com/metrics20/go-metrics20/carbon20"
	logging "github.com/op/go-logging"
	"github.com/graphite-ng/carbon-relay-ng/aggregator"
	"github.com/graphite-ng/carbon-relay-ng/badmetrics"
	"github.com/graphite-ng/carbon-relay-ng/cfg"
	"github.com/graphite-ng/carbon-relay-ng/destination"
	"github.com/graphite-ng/carbon-relay-ng/input"
	"github.com/graphite-ng/carbon-relay-ng/route"
	"github.com/graphite-ng/carbon-relay-ng/stats"
	tbl "github.com/graphite-ng/carbon-relay-ng/table"
	"github.com/graphite-ng/carbon-relay-ng/ui/telnet"
	"github.com/graphite-ng/carbon-relay-ng/ui/web"

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
	Version          = "unknown"
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
	aggregator.SetLogger(log)
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
		val := flag.Arg(0)
		if val == "version" {
			fmt.Printf("carbon-relay-ng %s (built with %s)\n", Version, runtime.Version())
			return
		}
		config_file = val
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

	config.Instance = os.Expand(config.Instance, expandVars)
	if len(config.Instance) == 0 {
		log.Error("instance identifier cannot be empty")
		os.Exit(1)
	}

	runtime.GOMAXPROCS(config.Max_procs)

	log.Notice("===== carbon-relay-ng instance '%s' starting. (version %s) =====\n", config.Instance, Version)
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

	go func() {
		sys := stats.Gauge("what=virtual_memory.unit=Byte")
		alloc := stats.Gauge("what=memory_allocated.unit=Byte")
		ticker := time.NewTicker(time.Second)
		var memstats runtime.MemStats
		for range ticker.C {
			runtime.ReadMemStats(&memstats)
			sys.Update(int64(memstats.Sys))
			alloc.Update(int64(memstats.Alloc))

		}
	}()

	if config.Instrumentation.Graphite_addr != "" {
		addr, err := net.ResolveTCPAddr("tcp", config.Instrumentation.Graphite_addr)
		if err != nil {
			log.Fatal(err)
		}
		go metrics.Graphite(metrics.DefaultRegistry, time.Duration(config.Instrumentation.Graphite_interval)*time.Millisecond, "", addr)
	}

	maxAge, err := time.ParseDuration(config.Bad_metrics_max_age)
	if err != nil {
		log.Error("could not parse badMetrics max age")
		log.Error(err.Error())
		os.Exit(1)
	}
	badMetrics = badmetrics.New(maxAge)

	log.Notice("initializing routing table...")

	table, err := tbl.InitFromConfig(config)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
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

	if config.Amqp.Amqp_enabled == true {
		go input.StartAMQP(config, table, badMetrics)
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

func expandVars(in string) (out string) {
	switch in {
	case "HOST":
		hostname, _ := os.Hostname()
		// in case hostname is an fqdn or has dots, only take first part
		parts := strings.SplitN(hostname, ".", 2)
		return parts[0]
	default:
		return ""
	}
}
