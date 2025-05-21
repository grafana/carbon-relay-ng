package route

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"strconv"
	"strings"

	"github.com/Dieterbe/go-metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	dest "github.com/grafana/carbon-relay-ng/destination"
	"github.com/grafana/carbon-relay-ng/matcher"
	"github.com/grafana/carbon-relay-ng/stats"
	log "github.com/sirupsen/logrus"
)

// Publishes data points to the native AWS metrics service: CloudWatch
type CloudWatch struct {
	awsProfile         string
	awsRegion          string
	awsNamespace       string
	awsDimensions      []types.Dimension
	storageResolution  int32
	client             *cloudwatch.Client
	putMetricDataInput cloudwatch.PutMetricDataInput

	baseRoute
	buf      chan []byte
	blocking bool
	dispatch func(chan []byte, []byte, metrics.Gauge, metrics.Counter)

	bufSize      int // amount of messages we can buffer up. each message is about 100B. so 1e7 is about 1GB.
	flushMaxSize int
	flushMaxWait time.Duration

	numOut                metrics.Counter // metrics successfully written to CloudWatch
	numCloudWatchMessages metrics.Counter // number of messages submitted to CloudWatch
	numErrFlush           metrics.Counter
	numDropBuffFull       metrics.Counter   // metric drops due to queue full
	numParseError         metrics.Counter   // metrics that failed destination.ParseDataPoint()
	durationTickFlush     metrics.Timer     // only updated after successful flush
	tickFlushSize         metrics.Histogram // only updated after successful flush
	numBuffered           metrics.Gauge
	bufferSize            metrics.Gauge
}

// NewCloudWatch creates a route that writes metrics to the AWS service CloudWatch
// We will automatically run the route and the destination
func NewCloudWatch(key string, matcher matcher.Matcher, awsProfile, awsRegion, awsNamespace string, awsDimensions [][]string, bufSize, flushMaxSize, flushMaxWait int, storageResolution int64, blocking bool) (Route, error) {

	r := &CloudWatch{
		awsProfile:         awsProfile,
		awsRegion:          awsRegion,
		awsNamespace:       awsNamespace,
		storageResolution:  int32(storageResolution),
		putMetricDataInput: cloudwatch.PutMetricDataInput{Namespace: aws.String(awsNamespace)},
		baseRoute:          baseRoute{"CloudWatch", sync.Mutex{}, atomic.Value{}, key},
		buf:                make(chan []byte, bufSize),
		blocking:           blocking,
		bufSize:            bufSize,
		flushMaxSize:       flushMaxSize,
		flushMaxWait:       time.Duration(flushMaxWait) * time.Millisecond,

		numOut:                stats.Counter("dest=cloudwatch" + ".unit=Metric.direction=out"),
		numCloudWatchMessages: stats.Counter("dest=cloudwatch" + "unit.Metric.what=CloudWatchMessagesPublished"),
		numErrFlush:           stats.Counter("dest=cloudwatch" + ".unit=Err.type=flush"),
		numParseError:         stats.Counter("dest=cloudwatch" + ".unit.Err.type=parse"),
		durationTickFlush:     stats.Timer("dest=cloudwatch" + ".what=durationFlush.type=ticker"),
		tickFlushSize:         stats.Histogram("dest=cloudwatch" + ".unit=B.what=FlushSize.type=ticker"),
		numBuffered:           stats.Gauge("dest=cloudwatch" + ".unit=Metric.what=numBuffered"),
		bufferSize:            stats.Gauge("dest=cloudwatch" + ".unit=Metric.what=bufferSize"),
		numDropBuffFull:       stats.Counter("dest=cloudwatch" + ".unit=Metric.action=drop.reason=queue_full"),
	}
	r.bufferSize.Update(int64(bufSize))

	if blocking {
		r.dispatch = dispatchBlocking
	} else {
		r.dispatch = dispatchNonBlocking
	}

	for _, dim := range awsDimensions {
		if len(dim) < 2 {
			log.Errorf("RouteCloudWatch: Dimension needs exactly 2 fields: name and val. got %v", dim)
			continue
		}
		r.awsDimensions = append(r.awsDimensions, types.Dimension{
			Name:  aws.String(dim[0]),
			Value: aws.String(dim[1])})
	}

	// Initialize AWS SDK configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(r.awsRegion),
	)
	if err != nil {
		return nil, err
	}

	// Create new CloudWatch client.
	r.client = cloudwatch.NewFromConfig(cfg)

	r.config.Store(baseConfig{matcher, make([]*dest.Destination, 0)})
	go r.run()
	return r, nil
}

func (r *CloudWatch) run() {
	ticker := time.NewTicker(r.flushMaxWait)
	var cnt int // number of metrics written to the payload buffer

	flush := func() {
		r.publish(r.putMetricDataInput, cnt)
		r.putMetricDataInput.MetricData = nil
		cnt = 0
	}

	for {
		select {
		case buf, ok := <-r.buf:
			if !ok {
				flush()
				return
			}
			r.numBuffered.Dec(1)

			// Parse metric data
			msg := strings.TrimSpace(string(buf))
			elements := strings.Fields(msg)
			if len(elements) != 3 {
				log.Error("RouteCloudWatch: need 3 fields")
				continue
			}
			val, err := strconv.ParseFloat(elements[1], 64)
			if err != nil {
				log.Errorf("RouteCloudWatch: unable to parse value: %s", err)
				continue
			}
			timestamp, err := strconv.ParseInt(elements[2], 10, 32)
			if err != nil {
				log.Errorf("RouteCloudWatch: unable to parse timestamp: %s", err)
				continue
			}

			// Write new metric data to slice
			newDatum := types.MetricDatum{
				MetricName:        aws.String(elements[0]),
				Timestamp:         aws.Time(time.Unix(timestamp, 0)),
				Value:             aws.Float64(val),
				StorageResolution: aws.Int32(r.storageResolution),
			}
			if len(r.awsDimensions) > 0 {
				newDatum.Dimensions = r.awsDimensions
			}
			r.putMetricDataInput.MetricData = append(r.putMetricDataInput.MetricData, newDatum)

			// flush if slice is likely to breach our size limit
			if len(r.putMetricDataInput.MetricData) >= r.flushMaxSize {
				flush()
			}

			cnt++
		case _ = <-ticker.C:
			if len(r.putMetricDataInput.MetricData) > 0 {
				flush()
			}
		}
	}
}

// publishes a batch to CloudWatch.
func (r *CloudWatch) publish(metricData cloudwatch.PutMetricDataInput, cnt int) {
	if cnt == 0 {
		return
	}
	start := time.Now()

	// Publish to CloudWatch!
	result, err := r.client.PutMetricData(context.TODO(), &metricData)
	if err != nil {
		log.Errorf("RouteCloudWatch: failed sending metric data: %s", err)
		r.numErrFlush.Inc(1)
		return
	}

	dataLength := int64(len(metricData.MetricData))
	dur := time.Since(start)
	r.numOut.Inc(int64(cnt))
	r.numCloudWatchMessages.Inc(1)
	r.durationTickFlush.Update(dur)
	r.tickFlushSize.Update(dataLength)
	// ex: "CloudWatch(key) publish success, count: 50000, size: 2139099, time: 1.288598 seconds"
	log.Debugf("CloudWatch(%s) publish success, count: %d, size: %d, time: %f ms, result: %s",
		r.Key(), cnt, dataLength, float64(dur)/float64(time.Millisecond), result)

}

// Dispatch is called to submit metrics. They will be in graphite 'plain' format no matter how they arrived.
func (r *CloudWatch) Dispatch(buf []byte) {
	r.dispatch(r.buf, buf, r.numBuffered, r.numDropBuffFull)
}

// Flush is not currently implemented
func (r *CloudWatch) Flush() error {
	// no-op. Flush() is currently not called by anything.
	return nil
}

// Shutdown stops the CloudWatch publisher and returns with the publisher has finished in-flight work
func (r *CloudWatch) Shutdown() error {
	close(r.buf)
	return nil
}
