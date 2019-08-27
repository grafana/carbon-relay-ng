package cfg

import (
	"errors"
	"fmt"
	"time"

	"github.com/Shopify/sarama"

	"github.com/mitchellh/mapstructure"

	"github.com/graphite-ng/carbon-relay-ng/encoding"
	"github.com/graphite-ng/carbon-relay-ng/input"
	"github.com/graphite-ng/carbon-relay-ng/util"
)

const (
	ListenerConfigType = "listener"
	KafkaConfigType    = "kafka"
)

const (
	decodingErrorFmt = "can't decode %s structure: %s"
	initErrorFmt     = "cant initialize %s structure: %s"
	decoderErrorFmt  = "can't initialize the mapstructure decoder: %s"
)

const (
	handlerErrorFmt = "can't initialize handler for %s"
)

const (
	kafkaInvalidAutoOffsetErrorFmt = "%s is not a valid auto_reset_offset in kafka config"
)

var (
	kafkaEmptyConsumerGroupError = errors.New("consumer_group_id can't be empty in kafka config")
	kafkaEmptyTopicError         = errors.New("topic can't be empty in kafka config")
	kafkaEmptyBrokersError       = errors.New("brokers can't be empty in kafka config")
	noInputError                 = errors.New("no inputs could be found")
)

type InputConfig interface {
	Handler() (encoding.FormatAdapter, error)
	Build() (input.Input, error)
}

type baseInputConfig struct {
	FormatOptions encoding.FormatOptions `mapstructure:"format_options,omitempty"`
	Format        encoding.FormatName    `mapstructure:"format,omitempty"`
}

func (bc baseInputConfig) Handler() (encoding.FormatAdapter, error) {
	return bc.Format.ToHandler(bc.FormatOptions)
}

type ListenerConfig struct {
	baseInputConfig `mapstructure:",squash"`
	Workers         int           `mapstructure:"workers,omitempty"`
	ListenAddr      string        `mapstructure:"listen_addr,omitempty"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout,omitempty"`
}

func (c *ListenerConfig) Build() (input.Input, error) {
	h, err := c.Handler()
	if err != nil {
		return nil, fmt.Errorf(handlerErrorFmt, fmt.Sprintf("listener[%s] config: %s", c.ListenAddr, err))
	}
	l := input.NewListener(c.ListenAddr, c.ReadTimeout, c.Workers, c.Workers, h)
	return l, nil
}

// for more informations about the fields, go look at https://github.com/segmentio/kafka-go/blob/master/reader.go#L291
type KafkaConfig struct {
	baseInputConfig     `mapstructure:",squash"`
	ID                  string        `mapstructure:"client_id,omitempty"`
	Brokers             []string      `mapstructure:"brokers,omitempty"`
	Topic               string        `mapstructure:"topic,omitempty"`
	ConsumerGroupID     string        `mapstructure:"consumer_group_id,omitempty"`
	QueueCapacity       int           `mapstructure:"queue_capacity,omitempty"`
	MinBytes            int           `mapstructure:"min_bytes,omitempty"`
	MaxBytes            int           `mapstructure:"max_bytes,omitempty"`
	CommitInterval      time.Duration `mapstructure:"commit_interval,omitempty"`
	SessionTimeout      time.Duration `mapstructure:"session_timeout,omitempty"`
	RebalanceTimeout    time.Duration `mapstructure:"rebalance_timeout,omitempty"`
	BackoffMin          time.Duration `mapstructure:"backoff_min,omitempty"`
	BackoffMax          time.Duration `mapstructure:"backoff_max,omitempty"`
	MaxAttempts         int           `mapstructure:"max_attempts,omitempty"`
	ReturnErrors        bool          `mapstructure:"return_errors,omitempty"`
	InitialOffsetOldest bool          `mapstructure:"initial_offset_oldest,omitempty"`
}

func (c *KafkaConfig) Build() (input.Input, error) {

	kafkaConfig := sarama.NewConfig()

	brokers := c.Brokers
	if brokers == nil || len(brokers) == 0 {
		return nil, kafkaEmptyBrokersError
	}

	consumerGroupID := c.ConsumerGroupID
	if consumerGroupID == "" {
		return nil, kafkaEmptyConsumerGroupError
	}

	topic := c.Topic
	if topic == "" {
		return nil, kafkaEmptyTopicError
	}

	// 0 Mean disabled
	kafkaConfig.Consumer.Offsets.CommitInterval = c.CommitInterval

	if c.RebalanceTimeout != 0 {
		kafkaConfig.Consumer.Group.Rebalance.Timeout = c.RebalanceTimeout
	}
	if c.SessionTimeout != 0 {
		kafkaConfig.Consumer.Group.Session.Timeout = c.SessionTimeout
	}
	kafkaConfig.Consumer.Return.Errors = c.ReturnErrors

	kafkaConfig.Consumer.Fetch.Min = int32(util.MaxInt(c.MinBytes, 1))
	kafkaConfig.Consumer.Fetch.Max = int32(c.MaxBytes)
	kafkaConfig.Consumer.Offsets.Retry.Max = util.MaxInt(c.MaxAttempts, 3)
	kafkaConfig.Consumer.Group.Rebalance.Retry.Max = util.MaxInt(c.MaxAttempts, 3)
	kafkaConfig.Consumer.Offsets.Retry.Max = util.MaxInt(c.MaxAttempts, 3)

	backoffMin := c.BackoffMin
	backoffMax := c.BackoffMax

	if backoffMin == 0 {
		backoffMin = 100 * time.Millisecond
	}

	if backoffMax == 0 {
		backoffMax = 20 * backoffMin
	}

	if backoffMax < backoffMin {
		return nil, fmt.Errorf("backoff_max must be not inferior to backoff_min")
	}

	// Poor's man exponential backoff factor
	kafkaConfig.Consumer.Retry.BackoffFunc = func(retries int) time.Duration {
		var ns int64
		ns = ns << uint(retries)
		backoffMin := int64(backoffMin)
		backoffMax := int64(backoffMax)
		if ns < backoffMin {
			ns = backoffMin
		} else if ns > backoffMax {
			ns = backoffMax
		}
		return time.Duration(ns)
	}

	kafkaConfig.ChannelBufferSize = util.MaxInt(c.QueueCapacity, 1000)

	kafkaConfig.ClientID = c.ID

	if c.InitialOffsetOldest {
		kafkaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	}

	kafkaConfig.Version = sarama.V2_2_0_0

	h, err := c.Handler()
	if err != nil {
		return nil, fmt.Errorf(handlerErrorFmt, fmt.Sprintf("kafka config: %s", err))
	}
	return input.NewKafka(brokers, topic, consumerGroupID, kafkaConfig, h), nil
}

func (c *Config) ProcessInputConfig() error {
	if c.InputsRaw == nil || len(c.InputsRaw) == 0 {
		return fmt.Errorf("no input provided")
	}
	inputs := make([]input.Input, len(c.InputsRaw))
	for i := 0; i < len(c.InputsRaw); i++ {
		configMap := c.InputsRaw[i]
		tRaw, ok := configMap["type"]
		if !ok {
			return fmt.Errorf("type must be set")
		}
		t, ok := tRaw.(string)
		if !ok {
			return fmt.Errorf("type must be a string")
		}

		var n InputConfig
		switch t {
		case KafkaConfigType:
			n = &KafkaConfig{}
		case ListenerConfigType:
			n = &ListenerConfig{Workers: 1, ReadTimeout: 2 * time.Minute}
		case "":
			return fmt.Errorf("input type can't be \"\"")
		default:
			return fmt.Errorf("unknown input type: \"%s\"", configMap["type"])
		}
		// To avoid being catched by the strict decoding
		delete(configMap, "type")

		d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			ErrorUnused:      true,
			WeaklyTypedInput: true,
			Result:           &n,
			DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		})
		if err != nil {
			return fmt.Errorf(decoderErrorFmt, err)
		}
		err = d.Decode(configMap)
		if err != nil {
			return fmt.Errorf(decodingErrorFmt, t, err)
		}
		l, err := n.Build()
		if err != nil {
			return fmt.Errorf(initErrorFmt, t, err)
		}
		inputs[i] = l
	}
	if c.NoInputError && len(inputs) == 0 {
		return noInputError
	}
	c.Inputs = inputs
	return nil
}
