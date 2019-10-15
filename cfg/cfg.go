package cfg

import (
	"time"

	"github.com/graphite-ng/carbon-relay-ng/input"
)

type Config struct {
	InputsRaw []map[string]interface{} `toml:"inputs, omitempty"`
	Inputs    []input.Input            `toml:"-"`

	Admin_addr          string
	Http_addr           string
	Spool_dir           string
	Max_procs           int
	First_only          bool
	Init                Init
	Instance            string
	Log_level           string
	Bad_metrics_max_age string
	Pid_file            string
	BlackList           []string
	Aggregation         []Aggregation
	Route               []Route
	Rewriter            []Rewriter

	// Should we crash if no input can be initialized ?
	NoInputError bool

	// Should we crash if no routes can be initialized ?
	NoRouteError bool
}

func NewConfig() Config {
	return Config{}
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

	Kafka *KafkaRouteConfig

	BgMetadata *BgMetadataRouteConfig `toml:"bg_metadata,omitempty"`

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

type KafkaRouteConfig struct {
	// No need to explain theses
	Brokers      []string      `toml:"brokers,omitempty"`
	Topic        string        `toml:"topic,omitempty"` // also used by Google PubSub
	Codec        string        `toml:"codec,omitempty"` // also used by Google PubSub
	PartitionBy  string        `toml:"partition_by,omitempty"`
	BatchSize    int           `toml:"batch_size,omitempty"`
	BatchBytes   int           `toml:"batch_bytes,omitempty"`
	BatchTimeout time.Duration `toml:"batch_timeout,omitempty"`
	RequiredAcks int           `toml:"required_acks,omitempty"`
	Synchronous  bool          `toml:"synchronous_acks,omitempty"`
	// FireAndForget bool          `toml:"fire_and_forget,omitempty"` <- This will be the default for now as we don't need any consistency
	HashBalance   bool `toml:"hashing_balancing,omitempty"`
	QueueCapacity int  `toml:"queue_capacity,omitempty"`
}

type BgMetadataRouteConfig struct {
	// TODO Add option to configure all bloom filter parameters
	// TODO Add additional configuration to for cassandra
	ShardingFactor int     `toml:"sharding_factor,omitempty"` // number of shards handling metrics
	FilterSize     uint    `toml:"filter_size,omitempty"`     // max total number of metrics
	FaultTolerance float64 `toml:"fault_tolerance,omitempty"` // value 0.0 - 1.0
	ClearInterval  string  `toml:"clear_interval,omitempty"`  // frequency of filter clearing
	ClearWait      string  `toml:"clear_wait,omitempty"`      // wait time between each filter clear. defaults to clear_wait/sharding_factor
	Cache          string  `toml:"cache,omitempty"`           // location of filter storage on disk; feature not enabled if path not provided
}

type Rewriter struct {
	Old string
	New string
	Not string
	Max int
}

type Init struct {
	Cmds []string
}
