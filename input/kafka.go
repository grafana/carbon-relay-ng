package input

import (
	"context"
	"github.com/Shopify/sarama"
	"github.com/graphite-ng/carbon-relay-ng/cfg"
	log "github.com/sirupsen/logrus"
	"strings"
)

type Kafka struct {
	config     cfg.Config
	dispatcher Dispatcher
	client     sarama.ConsumerGroup
	consumer   Consumer
	ctx        context.Context
	closed chan bool
}

func (kafka *Kafka) Name() string {
	return "kafka"
}

func (kafka *Kafka) Start() error {
	kafka.consumer.ready = make(chan bool, 0)

	go func() {
		for err := range kafka.client.Errors() {
			log.Errorln("kafka input error ", err)
		}
	}()
	go func(c chan bool) {
		for {
			select {
			case <-c:
				return
			default:
			}
			err := kafka.client.Consume(kafka.ctx, strings.Fields(kafka.config.Kafka.Kafka_topic), &kafka.consumer)
			if err != nil {
				log.Errorln("kafka input error Consume method ", err)
			}
			kafka.consumer.ready = make(chan bool, 0)
		}
	}(kafka.closed)
	<-kafka.consumer.ready // Await till the consumer has been set up
	log.Infoln("Sarama consumer up and running!...")
	return nil

}
func (kafka *Kafka) close(){
	err := kafka.client.Close()
	if err != nil {
		log.Errorln("kafka input closed with errors.", err)
	} else {
		log.Infoln("kafka input closed correctly.")
	}
}

func (kafka *Kafka) Stop() bool {
	close(kafka.closed)
	kafka.close()
	return true
}

func NewKafka(config cfg.Config, dispatcher Dispatcher) *Kafka {
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Consumer.Return.Errors = true
	kafkaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	kafkaConfig.Version = sarama.V2_2_0_0
	consumer := Consumer{}
	consumer.dispatcher = dispatcher

	client, err := sarama.NewConsumerGroup(config.Kafka.Kafka_brokers, config.Kafka.Kafka_consumer_group, kafkaConfig)
	if err != nil {
		log.Errorln("kafka input init failed", err)
	} else {
		log.Infoln("kafka input init correctly")
	}

	return &Kafka{
		consumer:   consumer,
		client:     client,
		config:     config,
		dispatcher: dispatcher,
		ctx:        context.Background(),
		closed: make(chan bool),
	}
}

// Consumer represents a Sarama consumer group consumer
type Consumer struct {
	ready      chan bool
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
		log.Traceln("metric value:", string(message.Value))
		consumer.dispatcher.Dispatch(message.Value)
		session.MarkMessage(message, "")
	}
	return nil
}
