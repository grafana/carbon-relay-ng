package input

import (
	"context"
	"github.com/Shopify/sarama"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"github.com/graphite-ng/carbon-relay-ng/cfg"
)



type Kafka struct {
	config cfg.Config
	dispatcher Dispatcher
	client sarama.ConsumerGroup
	consumer Consumer
	ctx context.Context
}

func (kafka *Kafka) Name() string {
	return "kafka"
}


func (kafka *Kafka) Start() error {
	go func() {
		for {
			kafka.consumer.ready = make(chan bool, 0)
			err := kafka.client.Consume(kafka.ctx, strings.Fields(kafka.config.Kafka.Kafka_topic), &kafka.consumer)
			if err != nil {
				panic(err)
			}
		}
	}()
	<-kafka.consumer.ready // Await till the consumer has been set up
	log.Infoln("Sarama consumer up and running!...")

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)

	<-sigterm // Await a sigterm signal before safely closing the consumer

	err := kafka.client.Close()

	return err
}

func (kafka *Kafka) Stop() bool {
	err := kafka.client.Close()
	if err != nil {
		panic(err)
	}
	return true
}

func NewKafka(config cfg.Config, dispatcher Dispatcher) *Kafka {
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Consumer.Return.Errors = true
	kafkaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	kafkaConfig.Version = sarama.V2_2_0_0
	consumer := Consumer{}
	consumer.dispatcher=dispatcher

	ctx := context.Background()

	client, err := sarama.NewConsumerGroup(config.Kafka.Kafka_brokers, config.Kafka.Kafka_consumer_group, kafkaConfig)
	if err != nil {
		panic(err)
	}

	return &Kafka{
		ctx: ctx,
		consumer: consumer,
		client: client,
		config:     config,
		dispatcher: dispatcher,
	}
}
// Consumer represents a Sarama consumer group consumer
type Consumer struct {
	ready chan bool
	dispatcher Dispatcher
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (consumer *Consumer) Setup(sarama.ConsumerGroupSession) error {
	// Mark the consumer as ready
	close(consumer.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (consumer *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (consumer *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	// NOTE:
	// Do not move the code below to a goroutine.
	// The `ConsumeClaim` itself is called within a goroutine, see:
	// https://github.com/Shopify/sarama/blob/master/consumer_group.go#L27-L29
	for message := range claim.Messages() {
		log.Traceln("metric value:",string(message.Value))
		consumer.dispatcher.Dispatch(message.Value)
		session.MarkMessage(message, "")
	}
	return nil
}

