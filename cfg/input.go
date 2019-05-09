package cfg

import (
	"errors"
	"fmt"
	"time"

	"github.com/Shopify/sarama"

	"github.com/davecgh/go-spew/spew"

	"github.com/mitchellh/mapstructure"

	"github.com/graphite-ng/carbon-relay-ng/formats"
	"github.com/graphite-ng/carbon-relay-ng/input"
)

const (
	ListenerConfigType = "listener"
	KafkaConfigType    = "kafka"
)

const (
	decodingErrorFmt = "error while decoding %s structure: %s"
	initErrorFmt     = "error while initializing %s structure: %s"
)

const (
	handlerErrorFmt = "can't initialize handler for: %s"
)

const (
	kafkaInvalidAutoOffsetErrorFmt = "%s is not a valid auto_reset_offset in kafka config"
)

var (
	kafkaEmptyConsumerGroupError = errors.New("consumer_group can't be empty in kafka config")
	kafkaEmptyTopicError         = errors.New("topic can't be empty in kafka config")
	kafkaEmptyBrokersError       = errors.New("brokers can't be empty in kafka config")
)

type InputConfig interface {
	Type() string
	Format() formats.FormatHandler
	Build() (*input.Input, error)
}

type baseInputConfig struct {
	FormatOptions formats.FormatOptions `mapstructure:"format_options,omitempty"`
	Format        formats.FormatName    `mapstructure:"format,omitempty"`
}

func (bc baseInputConfig) Handler() (formats.FormatHandler, error) {
	return bc.Format.ToHandler(bc.FormatOptions)
}

type ListenerConfig struct {
	baseInputConfig `mapstructure:",squash"`
	Workers         int           `mapstructure:"workers,omitempty"`
	ListenAddr      string        `mapstructure:"listen_addr,omitempty"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout,omitempty"`
}

func (c ListenerConfig) Build() (*input.Listener, error) {
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

func (c KafkaConfig) Build() (*input.Kafka, error) {
	// Validate offset
	var offset int64
	switch c.AutoOffsetReset {
	case "newest":
		offset = sarama.OffsetNewest
	case "earliest":
		offset = sarama.OffsetOldest
	default:
		return nil, fmt.Errorf(kafkaInvalidAutoOffsetErrorFmt, offset)
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
		switch configMap["type"].(string) {
		case ListenerConfigType:
			n := ListenerConfig{Workers: 1, ReadTimeout: 2 * time.Minute}
			d, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				WeaklyTypedInput: true,
				Result:           &n,
				DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
			})

			err := d.Decode(configMap)
			spew.Dump(n)
			spew.Dump(configMap)
			if err != nil {
				return fmt.Errorf(decodingErrorFmt, configMap["type"], err)
			}
			l, err := n.Build()
			if err != nil {
				return fmt.Errorf(initErrorFmt, configMap["type"], err)
			}
			inputs[i] = l
		case "":
			return fmt.Errorf("input type can't be \"\"")
		default:
			return fmt.Errorf("unknown input type: \"%s\"", configMap["type"])
		}
	}
	c.Inputs = inputs
	return nil
}
