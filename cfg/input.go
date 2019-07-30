package cfg

import (
	"errors"
	"fmt"
	"time"

	"github.com/Shopify/sarama"

	"github.com/mitchellh/mapstructure"

	"github.com/graphite-ng/carbon-relay-ng/encoding"
	"github.com/graphite-ng/carbon-relay-ng/input"
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
	kafkaEmptyConsumerGroupError = errors.New("consumer_group can't be empty in kafka config")
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

type KafkaConfig struct {
	baseInputConfig `mapstructure:",squash"`
	ID              string   `mapstructure:"client_id,omitempty"`
	Brokers         []string `mapstructure:"brokers,omitempty"`
	Topic           string   `mapstructure:"topic,omitempty"`
	AutoOffsetReset string   `mapstructure:"auto_offset_reset,omitempty"`
	ConsumerGroup   string   `mapstructure:"topic,omitempty"`
}

func (c *KafkaConfig) Build() (input.Input, error) {
	// Validate offset
	var offset int64
	switch c.AutoOffsetReset {
	case "newest":
		offset = sarama.OffsetNewest
	case "earliest":
		offset = sarama.OffsetOldest
	default:
		return nil, fmt.Errorf(kafkaInvalidAutoOffsetErrorFmt, c.AutoOffsetReset)
	}

	if len(c.Brokers) == 0 {
		return nil, kafkaEmptyBrokersError
	}
	if c.ConsumerGroup == "" {
		return nil, kafkaEmptyConsumerGroupError
	}
	if c.Topic == "" {
		return nil, kafkaEmptyTopicError
	}

	h, err := c.Handler()
	if err != nil {
		return nil, fmt.Errorf(handlerErrorFmt, fmt.Sprintf("kafka config: %s", err))
	}
	l := input.NewKafka(c.ID, c.Brokers, c.Topic, offset, c.ConsumerGroup, h)
	return l, nil
}

func (c *Config) ProcessInputConfig() error {
	if c.InputsRaw == nil || len(c.InputsRaw) == 0 {
		return fmt.Errorf("no input provided")
	}
	inputs := make([]input.Input, len(c.InputsRaw))
	for i := 0; i < len(c.InputsRaw); i++ {
		configMap := c.InputsRaw[i]
		var n InputConfig
		switch configMap["type"].(string) {
		case KafkaConfigType:
			n = &KafkaConfig{AutoOffsetReset: "earliest"}
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
			return fmt.Errorf(decodingErrorFmt, configMap["type"], err)
		}
		l, err := n.Build()
		if err != nil {
			return fmt.Errorf(initErrorFmt, configMap["type"], err)
		}
		inputs[i] = l
	}
	if c.NoInputError && len(inputs) == 0 {
		return noInputError
	}
	c.Inputs = inputs
	return nil
}
