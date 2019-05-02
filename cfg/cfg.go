package cfg

import (
	"time"

	"github.com/graphite-ng/carbon-relay-ng/validate"
	m20 "github.com/metrics20/go-metrics20/carbon20"
)

type Config struct {
	Listen_addr             string
	TCP_workers             int
	UDP_workers             int
	Plain_read_timeout      Duration
	Pickle_addr             string
	Pickle_read_timeout     Duration
	Admin_addr              string
	Http_addr               string
	Spool_dir               string
	Kafka                   Kafka
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
	BlackList               []string
	Aggregation             []Aggregation
	Route                   []Route
	Rewriter                []Rewriter
}

func NewConfig() Config {
	return Config{
		TCP_workers: 1,
		UDP_workers: 1,
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
	Function string
	Regex    string
	Prefix   string
	Substr   string
	Format   string
	Cache    bool
	Interval int
	Wait     int
	DropRaw  bool
}

type Route struct {
	Key          string   `toml:"key,omitempty"`
	Type         string   `toml:"type,omitempty"`
	Prefix       string   `toml:"prefix,omitempty"`
	Substr       string   `toml:"substr,omitempty"`
	Regex        string   `toml:"regex,omitempty"`
	Destinations []string `toml:"destinations,omitempty"`

	// grafanaNet & kafkaMdm & Google PubSub
	SchemasFile  string `toml:"schemas_file,omitempty"`
	OrgId        int    `toml:"org_id,omitempty"`
	BufSize      int    `toml:"buf_size,omitempty"`
	FlushMaxNum  int    `toml:"flush_max_num,omitempty"`  // also used by CloudWatch
	FlushMaxWait int    `toml:"flush_max_wait,omitempty"` // also used by CloudWatch
	Timeout      int    `toml:"timeout,omitempty"`
	Blocking     bool   `toml:"blocking,omitempty"`

	// grafanaNet
	Addr        string `toml:"addr,omitempty"`
	ApiKey      string `toml:"api_key,omitempty"`
	Spool       bool   `toml:"spool,omitempty"`
	SslVerify   *bool  `toml:"ssl_verify,omitempty"`
	Concurrency int    `toml:"concurrency,omitempty"`

	// kafkaMdm
	Brokers     []string `toml:"brokers,omitempty"`
	Topic       string   `toml:"topic,omitempty"` // also used by Google PubSub
	Codec       string   `toml:"codec,omitempty"` // also used by Google PubSub
	PartitionBy string   `toml:"partition_by,omitempty"`

	// Google PubSub
	Project      string `toml:"project,omitempty"`
	Format       string `toml:"format,omitempty"`
	FlushMaxSize int    `toml:"flush_max_size,omitempty"`

	// CloudWatch
	Profile           string     `toml:"profile,omitempty"` // For local development
	Region            string     `toml:"region,omitempty"`
	Namespace         string     `toml:"namespace,omitempty"`  // For now fixed in config
	Dimensions        [][]string `toml:"dimensions,omitempty"` // For now fixed in config
	StorageResolution int64      `toml:"storage_resolution,omitempty"`

	// ConsistentHashing
	RoutingMutations map[string]string `toml:"routing_mutations,omitempty"`
	CacheSize        int               `toml:"cache_size,omitempty"` // In bytes
	// Note than the cache will be disabled if <= 0
	// Then it will minimize at 512KB. To optimize the cache, you need to set it to at least (n * 1024) with n being the max len of your key size
}

type Rewriter struct {
	Old string
	New string
	Not string
	Max int
}

type Amqp struct {
	Amqp_enabled   bool
	Amqp_host      string
	Amqp_port      int
	Amqp_vhost     string
	Amqp_user      string
	Amqp_password  string
	Amqp_exchange  string
	Amqp_queue     string
	Amqp_key       string
	Amqp_durable   bool
	Amqp_exclusive bool
}

type Kafka struct {
	Kafka_brokers           []string
	Kafka_topic             string
	Kafka_auto_offset_reset string
	Kafka_consumer_group    string
	Kafka_enabled           bool
}

type Init struct {
	Cmds []string
}

type instrumentation struct {
	Graphite_addr     string
	Graphite_interval int
}
