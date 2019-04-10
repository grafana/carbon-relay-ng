package route

import (
	"time"

	dest "github.com/graphite-ng/carbon-relay-ng/destination"
	"github.com/graphite-ng/carbon-relay-ng/matcher"

	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	log "github.com/sirupsen/logrus"
)

// Publishes data points to the native AWS metrics service: CloudWatch
type CloudWatch struct {
	awsProfile         string
	awsRegion          string
	awsNamespace       string
	awsDimensions      []*cloudwatch.Dimension
	storageResolution  int64
	session            *session.Session
	client             *cloudwatch.CloudWatch
	putMetricDataInput cloudwatch.PutMetricDataInput

	baseRoute
	buf      chan []byte
	blocking bool
	dispatch func(chan []byte, []byte)

	bufSize      int // amount of messages we can buffer up. each message is about 100B. so 1e7 is about 1GB.
	flushMaxSize int
	flushMaxWait time.Duration
}

// NewCloudWatch creates a route that writes metrics to the AWS service CloudWatch
// We will automatically run the route and the destination
func NewCloudWatch(key, prefix, sub, regex, awsProfile, awsRegion, awsNamespace string, awsDimensions [][]string, bufSize, flushMaxSize, flushMaxWait int, storageResolution int64, blocking bool) (Route, error) {
	m, err := matcher.New(prefix, sub, regex)
	if err != nil {
		return nil, err
	}

	r := &CloudWatch{
		awsProfile:         awsProfile,
		awsRegion:          awsRegion,
		awsNamespace:       awsNamespace,
		storageResolution:  storageResolution,
		putMetricDataInput: cloudwatch.PutMetricDataInput{Namespace: aws.String(awsNamespace)},
		baseRoute:          *newBaseRoute(key, "CloudWatch"),
		buf:                make(chan []byte, bufSize),
		blocking:           blocking,
		bufSize:            bufSize,
		flushMaxSize:       flushMaxSize,
		flushMaxWait:       time.Duration(flushMaxWait) * time.Millisecond,
	}
	r.rm.Buffer.Size.Set(float64(bufSize))

	if blocking {
		r.dispatch = r.dispatchBlocking
	} else {
		r.dispatch = r.dispatchNonBlocking
	}

	for _, dim := range awsDimensions {
		if len(dim) < 2 {
			log.Errorf("RouteCloudWatch: Dimension needs exactly 2 fields: name and val. got %v", dim)
			continue
		}
		r.awsDimensions = append(r.awsDimensions, &cloudwatch.Dimension{
			Name:  aws.String(dim[0]),
			Value: aws.String(dim[1])})
	}

	// Initialize a session at AWS
	r.session = session.Must(session.NewSessionWithOptions(session.Options{
		// For local development:
		// credentials from the shared credentials file ~/.aws/credentials
		// and configuration from the shared configuration file ~/.aws/config.
		// SharedConfigState:       session.SharedConfigEnable,
		// AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
		// Profile:                 r.awsProfile,
		Config: aws.Config{
			Region: aws.String(r.awsRegion),
		},
	}))

	// Create new CloudWatch client.
	r.client = cloudwatch.New(r.session)

	r.config.Store(baseConfig{*m, make([]*dest.Destination, 0)})
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
			r.rm.Buffer.BufferedMetrics.Dec()

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
			newDatum := &cloudwatch.MetricDatum{
				MetricName:        aws.String(elements[0]),
				Timestamp:         aws.Time(time.Unix(timestamp, 0)),
				Value:             aws.Float64(val),
				StorageResolution: aws.Int64(r.storageResolution),
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
	result, err := r.client.PutMetricData(&metricData)
	if err != nil {
		log.Errorf("RouteCloudWatch: failed sending metric data: %s", err)
		r.rm.Errors.WithLabelValues("flush").Inc()
		return
	}

	dataLength := int64(len(metricData.MetricData))
	dur := time.Since(start)
	r.rm.OutMetrics.Add(float64(cnt))
	r.rm.Buffer.ObserveFlush(dur, dataLength, "")
	log.Debugf("CloudWatch(%s) publish success, count: %d, size: %d, time: %f ms, result: %s",
		r.Key(), cnt, dataLength, float64(dur)/float64(time.Millisecond), result)

}

// Dispatch is called to submit metrics. They will be in graphite 'plain' format no matter how they arrived.
func (r *CloudWatch) Dispatch(buf []byte) {
	r.dispatch(r.buf, buf)
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

// Snapshot clones the current config for update operations
func (r *CloudWatch) Snapshot() Snapshot {
	return makeSnapshot(&r.baseRoute)
}
