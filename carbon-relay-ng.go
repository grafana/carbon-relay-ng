// carbon-relay-ng
// route traffic to anything that speaks the Graphite Carbon protocol (text or pickle)
// such as Graphite's carbon-cache.py, influxdb, ...
package main

import (
	"bufio"
	"encoding/json"
	"expvar"
	"flag"
	"fmt"
	"io"
	"net"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/BurntSushi/toml"
	"github.com/Dieterbe/go-metrics"
	"github.com/Dieterbe/go-metrics/exp"
	"github.com/graphite-ng/carbon-relay-ng/badmetrics"
	"github.com/graphite-ng/carbon-relay-ng/validate"
	m20 "github.com/metrics20/go-metrics20/carbon20"
	logging "github.com/op/go-logging"
	"github.com/rcrowley/goagain"
	//"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type MetricValidationLevel struct {
	Level m20.LegacyMetricValidation
}

func (m MetricValidationLevel) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Level.String())
}

func (l *MetricValidationLevel) UnmarshalText(text []byte) error {
	validationLevels := map[string]m20.LegacyMetricValidation{
		"strict": m20.Strict,
		"medium": m20.Medium,
		"none":   m20.None,
	}
	var err error
	var ok bool
	l.Level, ok = validationLevels[string(text)]
	if !ok {
		err = fmt.Errorf("Invalid validation level '%s'. Valid validation levels are 'strict', 'medium', and 'none'.", string(text))
	}
	return err
}

type Config struct {
	Listen_addr              string
	Admin_addr               string
	Http_addr                string
	Spool_dir                string
	max_procs                int
	First_only               bool
	Routes                   []*Route
	Init                     []string
	Instance                 string
	Log_level                string
	Instrumentation          instrumentation
	Bad_metrics_max_age      string
	Pid_file                 string
	Legacy_metric_validation MetricValidationLevel
	Validate_order           bool
}

type instrumentation struct {
	Graphite_addr     string
	Graphite_interval int
}

var (
	instance         string
	service          = "carbon-relay-ng"
	config_file      string
	config           Config
	to_dispatch      = make(chan []byte)
	table            *Table
	cpuprofile       = flag.String("cpuprofile", "", "write cpu profile to file")
	blockProfileRate = flag.Int("block-profile-rate", 0, "see https://golang.org/pkg/runtime/#SetBlockProfileRate")
	memProfileRate   = flag.Int("mem-profile-rate", 512*1024, "0 to disable. 1 for max precision (expensive!) see https://golang.org/pkg/runtime/#pkg-variables")
	numIn            metrics.Counter
	numInvalid       metrics.Counter
	numOutOfOrder    metrics.Counter
	badMetrics       *badmetrics.BadMetrics
)

var log = logging.MustGetLogger("carbon-relay-ng")

func init() {
	var format = "%{color}%{time:15:04:05.000000} ▶ %{level:.4s} %{color:reset} %{message}"
	logBackend := logging.NewLogBackend(os.Stderr, "", 0)
	logging.SetFormatter(logging.MustStringFormatter(format))
	logging.SetBackend(logBackend)

	exp.Exp(metrics.DefaultRegistry)

}

func accept(l *net.TCPListener, config Config) {
	for {
		c, err := l.AcceptTCP()
		if nil != err {
			log.Error(err.Error())
			break
		}
		go handle(c, config)
	}
}

func handle(c net.Conn, config Config) {
	defer c.Close()
	// TODO c.SetTimeout(60e9)
	r := bufio.NewReaderSize(c, 4096)
	for {

		// Note that everything in this loop should proceed as fast as it can
		// so we're not blocked and can keep processing
		// so the validation, the pipeline initiated via table.Dispatch(), etc
		// must never block.

		// note that we don't support lines longer than 4096B. that seems very reasonable..
		buf, _, err := r.ReadLine()

		if nil != err {
			if io.EOF != err {
				log.Error(err.Error())
			}
			break
		}

		buf_copy := make([]byte, len(buf), len(buf))
		copy(buf_copy, buf)
		numIn.Inc(1)

		key, _, ts, err := m20.ValidatePacket(buf, config.Legacy_metric_validation.Level)
		if err != nil {
			badMetrics.Add(key, buf, err)
			numInvalid.Inc(1)
			continue
		}

		if config.Validate_order {
			err = validate.Ordered(key, ts)
			if err != nil {
				badMetrics.Add(key, buf, err)
				numOutOfOrder.Inc(1)
				continue
			}
		}

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
	runtime.SetBlockProfileRate(*blockProfileRate)
	runtime.MemProfileRate = *memProfileRate

	// Default to strict validation
	config.Legacy_metric_validation.Level = m20.Strict

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

	runtime.GOMAXPROCS(config.max_procs)

	instance = config.Instance
	expvar.NewString("instance").Set(instance)
	expvar.NewString("service").Set(service)

	log.Notice("===== carbon-relay-ng instance '%s' starting. =====\n", instance)

	numIn = Counter("unit=Metric.direction=in")
	numInvalid = Counter("unit=Err.type=invalid")
	numOutOfOrder = Counter("unit=Err.type=out_of_order")
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
	table = NewTable(config.Spool_dir)
	log.Notice("initializing routing table...")
	for i, cmd := range config.Init {
		log.Notice("applying: %s", cmd)
		err = applyCommand(table, cmd)
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

	// Follow the goagain protocol, <https://github.com/rcrowley/goagain>.
	l, ppid, err := goagain.GetEnvs()
	if nil != err {
		laddr, err := net.ResolveTCPAddr("tcp", config.Listen_addr)
		if nil != err {
			log.Error(err.Error())
			os.Exit(1)
		}
		l, err = net.ListenTCP("tcp", laddr)
		if nil != err {
			log.Error(err.Error())

			os.Exit(1)
		}
		log.Notice("listening on %v/tcp", laddr)
		go accept(l.(*net.TCPListener), config)
	} else {
		log.Notice("resuming listening on %v/tcp", l.Addr())
		go accept(l.(*net.TCPListener), config)
		if err := goagain.KillParent(ppid); nil != err {
			log.Error(err.Error())
			os.Exit(1)
		}
		for {
			err := syscall.Kill(ppid, 0)
			if err != nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	udp_addr, err := net.ResolveUDPAddr("udp", config.Listen_addr)
	if nil != err {
		log.Error(err.Error())
		os.Exit(1)
	}
	udp_conn, err := net.ListenUDP("udp", udp_addr)
	if nil != err {
		log.Error(err.Error())
		os.Exit(1)
	}
	log.Notice("listening on %v/udp", udp_addr)
	go handle(udp_conn, config)

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
		log.Error(err.Error())
		os.Exit(1)
	}
}
