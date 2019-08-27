package input

import (
	"context"
	"fmt"
	"strings"

	"github.com/Shopify/sarama"
	"github.com/graphite-ng/carbon-relay-ng/encoding"
	"go.uber.org/zap"
)

type Kafka struct {
	BaseInput

	topic      string
	groupID    string
	dispatcher Dispatcher
	client     sarama.Client
	cg         sarama.ConsumerGroup
	ctx        context.Context
	closed     chan bool
	ready      chan bool
	logger     *zap.Logger
}

func (kafka *Kafka) Name() string {
	return "kafka"
}

func (k *Kafka) Start(d Dispatcher) error {
	k.Dispatcher = d

	k.ready = make(chan bool, 0)

	go func() {
		for err := range k.cg.Errors() {
			k.logger.Error("kafka input error ", zap.Error(err))
		}
	}()
	go func(c chan bool) {
		for {
			select {
			case <-c:
				return
			default:
			}
			err := k.cg.Consume(k.ctx, strings.Fields(k.topic), k)
			if err != nil {
				k.logger.Error("kafka input error Consume method ", zap.Error(err))
			}
			k.ready = make(chan bool, 0)
		}
	}(k.closed)
	<-k.ready // Await till the consumer has been set up
	k.logger.Info("Sarama consumer up and running!...")
	return nil

}
func (k *Kafka) close() {
	err := k.client.Close()
	if err != nil {
		k.logger.Error("kafka input closed with errors.", zap.Error(err))
	} else {
		k.logger.Info("kafka input closed correctly.")
	}
}

func (k *Kafka) Stop() error {
	close(k.closed)
	k.close()
	return nil
}

func NewKafka(brokers []string, topic string, consumerGroup string, kafkaConfig *sarama.Config, h encoding.FormatAdapter) *Kafka {

	logger := zap.L().With(zap.String("kafka_topic", topic), zap.String("kafka_consumer_group_id", consumerGroup))

	client, err := sarama.NewClient(brokers, kafkaConfig)
	if err != nil {
		logger.Fatal("kafka input init client failed", zap.Error(err))
	}
	cg, err := sarama.NewConsumerGroup(brokers, consumerGroup, kafkaConfig)
	if err != nil {
		logger.Fatal("kafka input init consumer group failed", zap.Error(err))
	} else {
		logger.Info("kafka input init correctly")
	}

	return &Kafka{
		BaseInput: BaseInput{handler: h, name: fmt.Sprintf("kafka[topic=%s;cg=%s;id=%s]", topic, consumerGroup, kafkaConfig.ClientID)},
		topic:     topic,
		client:    client,
		cg:        cg,
		groupID:   consumerGroup,
		ctx:       context.Background(),
		closed:    make(chan bool),
		logger:    logger,
	}
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (k *Kafka) Setup(session sarama.ConsumerGroupSession) error {
	// Mark the consumer as ready
	close(k.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (k *Kafka) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (k *Kafka) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		k.logger.Debug("metric value:", zap.ByteString("messageValue", message.Value))
		if err := k.handle(message.Value); err != nil {
			k.logger.Debug("invalid message from kafka", zap.ByteString("messageValue", message.Value))
		}
		session.MarkMessage(message, "")
	}
	return nil
}
