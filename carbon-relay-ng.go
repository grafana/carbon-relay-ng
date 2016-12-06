// carbon-relay-ng
// route traffic to anything that speaks the Graphite Carbon protocol (text or pickle)
// such as Graphite's carbon-cache.py, influxdb, ...
package main

import (
	"bufio"
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
	ogorek "github.com/kisielk/og-rek"
	//"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"encoding/binary"
	"bytes"
)

type Config struct {
	Listen_addr             string
	Pickle_addr             string
	Admin_addr              string
	Http_addr               string
	Spool_dir               string
	max_procs               int
	First_only              bool
	Routes                  []*Route
	Init                    []string
	Instance                string
	Log_level               string
	Instrumentation         instrumentation
	Bad_metrics_max_age     string
	Pid_file                string
	Validation_level_legacy validate.LevelLegacy
	Validation_level_m20    validate.LevelM20
	Validate_order          bool
}

type instrumentation struct {
	Graphite_addr     string
	Graphite_interval int
}

type Handler func(net.Conn, Config)

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

func accept(l *net.TCPListener, config Config, handler Handler) {
	for {
		c, err := l.AcceptTCP()
		if nil != err {
			log.Error(err.Error())
			break
		}
		go handler(c, config)
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

		numIn.Inc(1)

		key, _, ts, err := m20.ValidatePacket(buf, config.Validation_level_legacy.Level, config.Validation_level_m20.Level)
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

		table.Dispatch(buf)
	}
}

func handlePickle(c net.Conn, config Config) {
	defer c.Close()
	// TODO c.SetTimeout(60e9)
	r := bufio.NewReaderSize(c, 4096)
	ReadLoop:
		for {

			// Note that everything in this loop should proceed as fast as it can
			// so we're not blocked and can keep processing
			// so the validation, the pipeline initiated via table.Dispatch(), etc
			// must never block.

			var length uint32
			err := binary.Read(r, binary.BigEndian, &length)
			if nil != err {
				if io.EOF != err {
					log.Error("couldn't read payload length: " + err.Error())
				}
				break
			}

			lengthTotal := int(length)
			lengthRead := 0
			payload := make([]byte, lengthTotal, lengthTotal)
			for {
				tmpLengthRead, err := r.Read(payload[lengthRead:])
				if nil != err {
					log.Error("couldn't read payload: " + err.Error())
					break ReadLoop
				}
				lengthRead += tmpLengthRead
				if lengthRead == lengthTotal {
					break
				}
				if lengthRead > lengthTotal {
					log.Error(fmt.Sprintf("expected to read %d bytes, but read %d", length, lengthRead))
					break ReadLoop
				}
			}

			decoder := ogorek.NewDecoder(bytes.NewBuffer(payload))

			rawDecoded, err := decoder.Decode()
			if nil != err {
				if io.EOF != err {
					log.Error("error reading pickled data " + err.Error())
				}
				break
			}

			decoded, ok := rawDecoded.([]interface{})
			if !ok {
				log.Error(fmt.Sprintf("Unrecognized type %T for pickled data", rawDecoded))
				break
			}

			ItemLoop:
				for _, rawItem := range decoded {
					numIn.Inc(1)

					item, ok := rawItem.([]interface{})
					if !ok {
						log.Error(fmt.Sprintf("Unrecognized type %T for item", rawItem))
						numInvalid.Inc(1)
						continue
					}
					if len(item) != 2 {
						log.Error(fmt.Sprintf("item length must be 2, got %d", len(item)))
						numInvalid.Inc(1)
						continue
					}

					metric, ok := item[0].(string)
					if !ok {
						log.Error(fmt.Sprintf("item metric must be a string, got %T", item[0]))
						numInvalid.Inc(1)
						continue
					}

					data, ok := item[1].([]interface{})
					if !ok {
						log.Error(fmt.Sprintf("item data must be an array, got %T", item[1]))
						numInvalid.Inc(1)
						continue
					}
					if len(data) != 2 {
						log.Error(fmt.Sprintf("item data length must be 2, got %d", len(data)))
						numInvalid.Inc(1)
						continue
					}

					var value string
					switch data[1].(type) {
						case string:
							value = data[1].(string)
						case uint8, uint16, uint32, uint64, int8, int16, int32, int64:
							value = fmt.Sprintf("%d", data[1])
						case float32, float64:
							value = fmt.Sprintf("%f", data[1])
						default:
							log.Error(fmt.Sprintf("Unrecognized type %T for value", data[1]))
							numInvalid.Inc(1)
							continue ItemLoop
					}

					var timestamp string
					switch data[0].(type) {
						case string:
							timestamp = data[0].(string)
						case uint8, uint16, uint32, uint64, int8, int16, int32, int64:
							timestamp = fmt.Sprintf("%d", data[0])
						case float32, float64:
							timestamp = fmt.Sprintf("%.0f", data[0])
						default:
							log.Error(fmt.Sprintf("Unrecognized type %T for timestamp", data[0]))
							numInvalid.Inc(1)
							continue ItemLoop
					}

					buf := []byte(metric + " " + value + " " + timestamp)

					key, _, ts, err := m20.ValidatePacket(buf, config.Validation_level_legacy.Level, config.Validation_level_m20.Level)
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

					table.Dispatch(buf)
				}
		}
}

func usage() {
	fmt.Fprintln(
		os.Stderr,
		"Usage: carbon-relay-ng <path-to-config>",
	)
	flag.PrintDefaults()
}

func listen(config Config, addr string, handler Handler) (net.Listener, error) {
	// Follow the goagain protocol, <https://github.com/rcrowley/goagain>.
	l, ppid, err := goagain.GetEnvs()
	if nil != err {
		laddr, err := net.ResolveTCPAddr("tcp", addr)
		if nil != err {
			return nil, err
		}
		l, err = net.ListenTCP("tcp", laddr)
		if nil != err {
			return nil, err
		}
		log.Notice("listening on %v/tcp", laddr)
		go accept(l.(*net.TCPListener), config, handler)
	} else {
		log.Notice("resuming listening on %v/tcp", l.Addr())
		go accept(l.(*net.TCPListener), config, handler)
		if err := goagain.KillParent(ppid); nil != err {
			return nil, err
		}
		for {
			err := syscall.Kill(ppid, 0)
			if err != nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	udp_addr, err := net.ResolveUDPAddr("udp", addr)
	if nil != err {
		return nil, err
	}
	udp_conn, err := net.ListenUDP("udp", udp_addr)
	if nil != err {
		return nil, err
	}
	log.Notice("listening on %v/udp", udp_addr)
	go handler(udp_conn, config)

	return l, nil
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

	runtime.GOMAXPROCS(config.max_procs)

	instance = config.Instance
	expvar.NewString("instance").Set(instance)
	expvar.NewString("service").Set(service)

	log.Notice("===== carbon-relay-ng instance '%s' starting. =====\n", instance)

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

	var l net.Listener
	if config.Listen_addr != "" {
		l, err = listen(config, config.Listen_addr, handle)
		if nil != err {
			log.Error(err.Error())
			os.Exit(1)
		}
	}

	var lp net.Listener
	if config.Pickle_addr != "" {
		lp, err = listen(config, config.Pickle_addr, handlePickle)
		if nil != err {
			log.Error(err.Error())
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

	if nil != l {
		if err := goagain.AwaitSignals(l); nil != err {
			log.Error(err.Error())
			os.Exit(1)
		}
	}

	if nil != lp {
		if err := goagain.AwaitSignals(lp); nil != err {
			log.Error(err.Error())
			os.Exit(1)
		}
	}
}
