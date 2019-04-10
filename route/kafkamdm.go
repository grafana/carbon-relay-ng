package route

import (
	"fmt"
	"time"

	dest "github.com/graphite-ng/carbon-relay-ng/destination"
	"github.com/graphite-ng/carbon-relay-ng/matcher"
	instMetrics "github.com/graphite-ng/carbon-relay-ng/metrics"
	log "github.com/sirupsen/logrus"

	"github.com/Shopify/sarama"
	"github.com/graphite-ng/carbon-relay-ng/persister"
	"github.com/raintank/metrictank/cluster/partitioner"
	"gopkg.in/raintank/schema.v1"
)

type KafkaMdm struct {
	baseRoute
	saramaCfg   *sarama.Config
	producer    sarama.SyncProducer
	topic       string
	brokers     []string
	buf         chan []byte
	partitioner *partitioner.Kafka
	schemas     persister.WhisperSchemas
	blocking    bool
	dispatch    func(chan []byte, []byte)

	orgId int // organisation to publish data under

	bufSize      int // amount of messages we can buffer up. each message is about 100B. so 1e7 is about 1GB.
	flushMaxNum  int
	flushMaxWait time.Duration
}

// NewKafkaMdm creates a special route that writes to a grafana.net datastore
// We will automatically run the route and the destination
func NewKafkaMdm(key, prefix, sub, regex, topic, codec, schemasFile, partitionBy string, brokers []string, bufSize, orgId, flushMaxNum, flushMaxWait, timeout int, blocking bool) (Route, error) {
	m, err := matcher.New(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	schemas, err := getSchemas(schemasFile)
	if err != nil {
		return nil, err
	}

	r := &KafkaMdm{
		baseRoute: *newBaseRoute(key, "kafkaMdm"),
		topic:     topic,
		brokers:   brokers,
		buf:       make(chan []byte, bufSize),
		schemas:   schemas,
		blocking:  blocking,
		orgId:     orgId,

		bufSize:      bufSize,
		flushMaxNum:  flushMaxNum,
		flushMaxWait: time.Duration(flushMaxWait) * time.Millisecond,
	}
	r.rm.Buffer.Size.Set(float64(bufSize))

	if blocking {
		r.dispatch = r.dispatchBlocking
	} else {
		r.dispatch = r.dispatchNonBlocking
	}

	r.partitioner, err = partitioner.NewKafka(partitionBy)
	if err != nil {
		log.Fatalf("kafkaMdm %q: failed to initialize partitioner. %s", r.key, err)
	}

	// We are looking for strong consistency semantics.
	// Because we don't change the flush settings, sarama will try to produce messages
	// as fast as possible to keep latency low.
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas to ack the message
	config.Producer.Retry.Max = 10                   // Retry up to 10 times to produce the message
	config.Producer.Compression, err = getCompression(codec)
	if err != nil {
		log.Fatalf("kafkaMdm %q: %s", r.key, err)
	}
	config.Producer.Return.Successes = true
	config.Producer.Timeout = time.Duration(timeout) * time.Millisecond
	err = config.Validate()
	if err != nil {
		log.Fatalf("kafkaMdm %q: failed to validate kafka config. %s", r.key, err)
	}
	r.saramaCfg = config

	r.config.Store(baseConfig{*m, make([]*dest.Destination, 0)})
	go r.run()
	return r, nil
}

func (r *KafkaMdm) run() {
	metrics := make([]*schema.MetricData, 0, r.flushMaxNum)
	ticker := time.NewTicker(r.flushMaxWait)
	var err error

	for r.producer == nil {
		r.producer, err = sarama.NewSyncProducer(r.brokers, r.saramaCfg)
		if err == sarama.ErrOutOfBrokers {
			log.Warnf("kafkaMdm %q: %s", r.key, err)
			// sleep before trying to connect again.
			time.Sleep(time.Second)
		} else if err != nil {
			log.Fatalf("kafkaMdm %q: failed to initialize kafka producer. %s", r.key, err)
		}
	}
	log.Infof("kafkaMdm %q: now connected to kafka", r.key)

	// flushes the data to kafka and resets buffer.  blocks until it succeeds
	flush := func() {
		for {
			pre := time.Now()
			size := 0

			payload := make([]*sarama.ProducerMessage, len(metrics))

			for i, metric := range metrics {
				var data []byte
				data, err = metric.MarshalMsg(data[:])
				if err != nil {
					panic(err)
				}
				size += len(data)

				key, err := r.partitioner.GetPartitionKey(metric, nil)
				if err != nil {
					panic(err)
				}
				payload[i] = &sarama.ProducerMessage{
					Key:   sarama.ByteEncoder(key),
					Topic: r.topic,
					Value: sarama.ByteEncoder(data),
				}
			}
			err = r.producer.SendMessages(payload)

			diff := time.Since(pre)
			if err == nil {
				log.Debugf("KafkaMdm %q: sent %d metrics in %s - msg size %d", r.key, len(metrics), diff, size)
				r.rm.OutMetrics.Add(float64(len(metrics)))
				r.rm.OutBatches.Inc()
				r.rm.Buffer.ObserveFlush(diff, int64(size), instMetrics.FlushTypeTicker)
				metrics = metrics[:0]
				break
			}

			errors := make(map[error]int)
			for _, e := range err.(sarama.ProducerErrors) {
				r.rm.Errors.WithLabelValues("flush").Inc()
				errors[e.Err] += 1
			}
			for k, v := range errors {
				log.Warnf("KafkaMdm %q: seen %d times: %s", r.key, v, k)
			}

			log.Warnf("KafkaMdm %q: failed to submit data: %s will try again in 100ms. (this attempt took %s)", r.key, err, diff)

			time.Sleep(100 * time.Millisecond)
		}
	}
	for {
		select {
		case buf, ok := <-r.buf:
			if !ok {
				if len(metrics) != 0 {
					flush()
				}
				return
			}
			r.rm.Buffer.BufferedMetrics.Dec()
			md, err := parseMetric(buf, r.schemas, r.orgId)
			if err != nil {
				log.Errorf("KafkaMdm %q: %s", r.key, err)
				continue
			}
			md.SetId()
			metrics = append(metrics, md)
			if len(metrics) == r.flushMaxNum {
				flush()
			}
		case <-ticker.C:
			if len(metrics) != 0 {
				flush()
			}
		}
	}
}

func (r *KafkaMdm) Dispatch(buf []byte) {
	log.Tracef("kafkaMdm %q: sending to dest %v: %s", r.key, r.brokers, buf)
	r.dispatch(r.buf, buf)
}

func (r *KafkaMdm) Flush() error {
	//conf := r.config.Load().(Config)
	// no-op. Flush() is currently not called by anything.
	return nil
}

func (r *KafkaMdm) Shutdown() error {
	//conf := r.config.Load().(Config)
	close(r.buf)
	return nil
}

func (r *KafkaMdm) Snapshot() Snapshot {
	return makeSnapshot(&r.baseRoute)
}

func getCompression(codec string) (sarama.CompressionCodec, error) {
	switch codec {
	case "none":
		return sarama.CompressionNone, nil
	case "gzip":
		return sarama.CompressionGZIP, nil
	case "snappy":
		return sarama.CompressionSnappy, nil
	}
	return 0, fmt.Errorf("unknown compression codec %q", codec)
}
