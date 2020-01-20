package storage

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/elastic/go-elasticsearch/v6"
	"github.com/graphite-ng/carbon-relay-ng/storage/mocks"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSyncWritesMetricsMetadata(t *testing.T) {
	mockElasticSearchClient := &mocks.ElasticSearchClient{}
	registry := prometheus.NewRegistry()
	esc := NewBgMetadataElasticSearchConnector(mockElasticSearchClient, registry, 10, 0, "")

	response := http.Response{Body: ioutil.NopCloser(strings.NewReader("<response>")), StatusCode: 200}

	mockElasticSearchClient.On("Perform", mock.Anything).Return(&response, nil)

	for i := 0; i < 25; i++ {
		err := esc.UpdateMetricMetadata(createMetric())
		assert.Nil(t, err)
	}
	esc.Close()

	successes := getMetricValue(esc.UpdatedMetrics, map[string]string{"status": "success"})
	assert.Equal(t, successes, 20.0)
	mockElasticSearchClient.AssertExpectations(t)
}

func TestAsyncWritesMetricsMetadata(t *testing.T) {
	mockElasticSearchClient := &mocks.ElasticSearchClient{}
	registry := prometheus.NewRegistry()
	esc := NewBgMetadataElasticSearchConnector(mockElasticSearchClient, registry, 10, 0, "")

	response := http.Response{Body: ioutil.NopCloser(strings.NewReader("<response>")), StatusCode: 200}
	mockElasticSearchClient.On("Perform", mock.Anything).Return(&response, nil)
	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go updateMetricAsync(createMetric(), esc, &wg)
	}
	wg.Wait()
	esc.Close()

	successes := getMetricValue(esc.UpdatedMetrics, map[string]string{"status": "success"})
	assert.Equal(t, successes, 20.0)
	mockElasticSearchClient.AssertExpectations(t)
}

func TestHandlesFailureWhenWritingAMetricMetadata(t *testing.T) {
	mockElasticSearchClient := &mocks.ElasticSearchClient{}
	registry := prometheus.NewRegistry()
	esc := NewBgMetadataElasticSearchConnector(mockElasticSearchClient, registry, 1, 1, "")
	response := http.Response{Body: ioutil.NopCloser(strings.NewReader("<response>")), StatusCode: 200}
	mockElasticSearchClient.On("Perform", mock.Anything).Return(&response, nil).Twice() // getIndex
	mockElasticSearchClient.On("Perform", mock.Anything).Return(&http.Response{}, errors.New("<error>"))

	err := esc.UpdateMetricMetadata(createMetric())
	esc.Close()

	assert.Nil(t, err)
	failures := getMetricValue(esc.UpdatedMetrics, map[string]string{"status": "failure"})
	assert.Equal(t, failures, 1.0)
	mockElasticSearchClient.AssertExpectations(t)
}

func TestHandlesRetry(t *testing.T) {
	mockElasticSearchClient := &mocks.ElasticSearchClient{}
	registry := prometheus.NewRegistry()
	esc := NewBgMetadataElasticSearchConnector(mockElasticSearchClient, registry, 1, 2, "")

	response := http.Response{Body: ioutil.NopCloser(strings.NewReader("<response>")), StatusCode: 200}
	badResponse := http.Response{Body: ioutil.NopCloser(strings.NewReader("<response>")), StatusCode: 400}
	mockElasticSearchClient.On("Perform", mock.Anything).Return(&response, nil).Twice() // getIndex
	mockElasticSearchClient.On("Perform", mock.Anything).Return(&badResponse, nil).Twice()
	mockElasticSearchClient.On("Perform", mock.Anything).Return(&response, nil).Once()

	err := esc.UpdateMetricMetadata(createMetric())
	assert.Nil(t, err)

	failures := getMetricValue(esc.UpdatedMetrics, map[string]string{"status": "failure"})
	successes := getMetricValue(esc.UpdatedMetrics, map[string]string{"status": "success"})
	httpErrors := getMetricValue(esc.HTTPErrors, map[string]string{"code": "400"})

	assert.Equal(t, 0.0, failures)
	assert.Equal(t, 1.0, successes)
	assert.Equal(t, 2.0, httpErrors)
	mockElasticSearchClient.AssertExpectations(t)
}

func getMetricValue(counterVec *prometheus.CounterVec, labels prometheus.Labels) float64 {
	counter, _ := counterVec.GetMetricWith(labels)
	metric := &dto.Metric{}
	_ = counter.Write(metric)
	return *metric.Counter.Value
}

func updateMetricAsync(m Metric, es *BgMetadataElasticSearchConnector, wg *sync.WaitGroup) {
	defer wg.Done()
	es.UpdateMetricMetadata(m)
}

func createMetricWithName(name string) Metric {
	metadata := MetricMetadata{
		aggregator:         "",
		carbonXfilesfactor: "",
		retention:          "",
	}
	return NewMetric(name, metadata, nil)
}

func createMetric() Metric {
	return createMetricWithName("a.b.c")
}

func createRandomMetric() Metric {
	length := 10
	var b strings.Builder
	for i := 0; i < length-1; i++ {
		b.WriteString(fmt.Sprintf("%d.", rand.Uint32()))
	}
	b.WriteString(fmt.Sprintf("%d", rand.Uint32()))

	return createMetricWithName(b.String())
}

func WriteRandomMetrics(numMetrics int, esc *BgMetadataElasticSearchConnector) {
	var err error
	for i := 0; i < numMetrics; i++ {
		err = esc.UpdateMetricMetadata(createRandomMetric())
		if err != nil {
			log.Fatal(err)
		}
	}
}

func benchmarkWrites(b *testing.B, esc *BgMetadataElasticSearchConnector, numMetrics int) {
	WriteRandomMetrics(numMetrics, esc)
}

const numMetrics = 10000

var es *elasticsearch.Client = nil

func BenchmarkWritesWithBulkSize100(b *testing.B) {
	fmt.Printf("bb %d\n", b.N)
	rand.Seed(0)
	es, err := GetClient()
	if err != nil {
		b.Fail()
	}
	esc := NewBgMetadataElasticSearchConnector(es, prometheus.DefaultRegisterer, 100, 0, "")
	for n := 0; n < b.N; n++ {
		benchmarkWrites(b, esc, numMetrics)
	}
}

func GetClient() (*elasticsearch.Client, error) {
	if es == nil {
		fmt.Printf("creating\n")
		es, err := CreateElasticSearchClient("http://localhost:9200", "", "")
		return es, err
	}
	fmt.Printf("cache\n")

	return es, nil
}
