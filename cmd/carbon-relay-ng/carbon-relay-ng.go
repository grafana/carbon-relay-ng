// carbon-relay-ng
// route traffic to anything that speaks the Graphite Carbon protocol (text or pickle)
// such as Graphite's carbon-cache.py, influxdb, ...
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path"
	"runtime"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/graphite-ng/carbon-relay-ng/badmetrics"
	"github.com/graphite-ng/carbon-relay-ng/cfg"
	tbl "github.com/graphite-ng/carbon-relay-ng/table"
	"github.com/graphite-ng/carbon-relay-ng/ui/telnet"
	"github.com/graphite-ng/carbon-relay-ng/ui/web"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"strconv"
	"strings"
	"time"
)

var (
	config_file     string
	config          = cfg.NewConfig()
	to_dispatch     = make(chan []byte)
	shutdownTimeout = time.Second * 30 // how long to wait for shutdown
	table           *tbl.Table
	enablePprof     = flag.Bool("enable-pprof", false, "Will enable debug endpoints on /debug/pprof/")
	badMetrics      *badmetrics.BadMetrics
	Version         = "unknown"
)

func usage() {
	header := `Usage:
        carbon-relay-ng version
        carbon-relay-ng <path-to-config>
	`
	fmt.Fprintln(os.Stderr, header)
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	config_file = "/etc/carbon-relay-ng.ini"
	if 1 == flag.NArg() {
		val := flag.Arg(0)
		if val == "version" {
			fmt.Printf("carbon-relay-ng %s (built with %s)\n", Version, runtime.Version())
			return
		}
		config_file = val
	}

	meta, err := toml.DecodeFile(config_file, &config)
	if err != nil {
		log.Fatalf("Invalid config file %q: %s", config_file, err.Error())
	}

	if len(meta.Undecoded()) > 0 {
		log.Fatalf("Unknown configuration keys in %s: %q", config_file, meta.Undecoded())
	}

	// expect logger configuration in the same directory than config file
	configLogger := path.Dir(config_file)
	configLogger = path.Join(configLogger, "logger.yaml")
	zapConfig := zap.Config{}
	zapConfigFile, err := ioutil.ReadFile(configLogger)
	if err != nil {
		// file doesn't exists, default to developement config
		zapConfig = zap.NewDevelopmentConfig()
	} else {
		err = yaml.Unmarshal(zapConfigFile, zapConfig)
		if err != nil {
			log.Fatalf("Cannot unmarshall logger configuration %v: %v", zapConfigFile, err)
		}
	}
	logger, _ := zapConfig.Build()
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	log := zap.S()

	config.Instance = os.Expand(config.Instance, expandVars)
	if len(config.Instance) == 0 {
		log.Error("instance identifier cannot be empty")
		os.Exit(1)
	}

	log.Infof("===== carbon-relay-ng instance '%s' starting. (version %s) =====", config.Instance, Version)

	if os.Getenv("GOMAXPROCS") == "" && config.Max_procs >= 1 {
		log.Debugf("setting GOMAXPROCS to %d", config.Max_procs)
		runtime.GOMAXPROCS(config.Max_procs)
	}

	if config.Pid_file != "" {
		f, err := os.Create(config.Pid_file)
		if err != nil {
			log.Fatalf("error creating pidfile: %s", err.Error())
		}
		_, err = f.Write([]byte(strconv.Itoa(os.Getpid())))
		if err != nil {
			log.Fatalf("error writing to pidfile: %s", err.Error())
		}
		f.Close()

	}

	err = config.ProcessInputConfig()
	if err != nil {
		log.Fatalf("can't initialize inputs config: %s", err)
	}

	log.Info("initializing routing table...")

	table, err := tbl.InitFromConfig(config, meta)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	tablePrinted := table.Print()
	log.Info("===========================")
	log.Info("========== TABLE ==========")
	log.Info("===========================")
	for _, line := range strings.Split(tablePrinted, "\n") {
		log.Info(line)
	}

	for _, in := range config.Inputs {
		err := in.Start(table)
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	if config.Admin_addr != "" {
		go func() {
			err := telnet.Start(config.Admin_addr, table)
			if err != nil {
				log.Fatalf("Error listening: %s", err.Error())
			}
		}()
	}

	if config.Http_addr != "" {
		go web.Start(config.Http_addr, config, table, *enablePprof)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig, ok := <-sigChan
	if ok {
		log.Infof("Received signal %q. Shutting down", sig)
	}
	for _, i := range config.Inputs {
		err = i.Stop()
		if err != nil {
			log.Warnf("failed to stop input %s: %s", i.Name(), err)
		}
	}
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
