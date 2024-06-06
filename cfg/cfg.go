package cfg

import (
	"time"

	"github.com/grafana/carbon-relay-ng/table"
	"github.com/grafana/carbon-relay-ng/validate"
	m20 "github.com/metrics20/go-metrics20/carbon20"
)

type Config struct {
	Listen_addr             string
	Plain_read_timeout      Duration
	Pickle_addr             string
	Pickle_read_timeout     Duration
	Admin_addr              string
	Http_addr               string
	Spool_dir               string
	Amqp                    Amqp
	Max_procs               int
	First_only              bool
	Init                    Init
	Instance                string
	Log_level               string
	Instrumentation         instrumentation
	Bad_metrics_max_age     string
	Pid_file                string
	Validation_level_legacy validate.LevelLegacy
	Validation_level_m20    validate.LevelM20
	Validate_order          bool
	BlackList               []string // support legacy configs
	BlockList               []string
	Aggregation             []Aggregation
	Route                   []Route
	Rewriter                []Rewriter
}

func NewConfig() Config {
	return Config{
		Plain_read_timeout: Duration{
			2 * time.Minute,
		},
		Pickle_read_timeout: Duration{
			2 * time.Minute,
		},
		Validation_level_legacy: validate.LevelLegacy{m20.MediumLegacy},
		Validation_level_m20:    validate.LevelM20{m20.MediumM20},
	}
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type Aggregation struct {
	Function  string
	Regex     string
	NotRegex  string
	Prefix    string
	NotPrefix string
	Substr    string
	Sub       string
	NotSub    string
	Format    string
	Cache     bool
	Interval  int
	Wait      int
	DropRaw   bool
}

type Route struct {
	Key          string
	Type         string
	Prefix       string
	NotPrefix    string
	Substr       string
	Sub          string
	NotSub       string
	Regex        string
	NotRegex     string
	Destinations []string

	// grafanaNet & kafkaMdm & Google PubSub
	SchemasFile  string
	OrgId        int
	BufSize      int
	FlushMaxNum  int // also used by CloudWatch
	FlushMaxWait int // also used by CloudWatch
	Timeout      int
	Blocking     bool

	// grafanaNet
	Addr             string
	AggregationFile  string
	ApiKey           string
	Spool            bool
	SslVerify        bool
	Concurrency      int
	ErrBackoffMin    int
	ErrBackoffFactor float64

	// kafkaMdm
	Brokers       []string
	Topic         string // also used by Google PubSub
	Codec         string // also used by Google PubSub
	PartitionBy   string
	TLSEnabled    bool
	TLSSkipVerify bool
	TLSClientCert string
	TLSClientKey  string
	SASLEnabled   bool
	SASLMechanism string
	SASLUsername  string
	SASLPassword  string

	// Google PubSub
	Project      string
	Format       string
	FlushMaxSize int

	// CloudWatch
	Profile           string // For local development
	Region            string
	Namespace         string     // For now fixed in config
	Dimensions        [][]string // For now fixed in config
	StorageResolution int64
}

type Rewriter struct {
	Old string
	New string
	Not string
	Max int
}

type Amqp struct {
	Amqp_enabled    bool
	Amqp_host       string
	Amqp_port       int
	Amqp_vhost      string
	Amqp_user       string
	Amqp_password   string
	Amqp_exchange   string
	Amqp_queue      string
	Amqp_key        string
	Amqp_durable    bool
	Amqp_exclusive  bool
	Amqp_heartbeat  int
	Amqp_retry      bool
	Amqp_retrydelay int
}

type Init struct {
	Cmds []string
}

type instrumentation struct {
	Graphite_addr     string
	Graphite_interval int
}

func (c Config) TableConfig() (table.TableConfig, error) {
	return table.NewTableConfig(c.Spool_dir, c.Bad_metrics_max_age, c.Validation_level_legacy, c.Validation_level_m20, c.Validate_order)
}
