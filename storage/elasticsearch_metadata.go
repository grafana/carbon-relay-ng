package storage

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v6"
	"github.com/elastic/go-elasticsearch/v6/esapi"
	"github.com/graphite-ng/carbon-relay-ng/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace                            = "elasticsearch"
	default_metrics_metadata_index       = "biggraphite_metrics"
	metrics_metadata_index_suffix_format = "_2006-01-02"
	mapping                              = `
{
"_doc": {
"properties": {
            "depth": { 
                "type": "long"
            },

            "name": {
                "type": "keyword",
                "ignore_above": 1024
            },
            "uuid": {
                "type": "keyword"
            },
            "config": {
                "type": "object"
            }
        },
			"dynamic_templates": [
            {
                "strings_as_keywords": {
                    "match": "p*",
                    "match_mapping_type": "string",
                    "mapping": {
                        "type": "keyword",
                        "ignore_above": 256,
                        "ignore_malformed": true
                    }
                }
            }
        ]
	}
}
`
	documentType = "_doc"
)

type BgMetadataElasticSearchConnector struct {
	client                  ElasticSearchClient
	UpdatedMetrics          *prometheus.CounterVec
	HTTPErrors              *prometheus.CounterVec
	WriteDurationMs         prometheus.Histogram
	DocumentBuildDurationMs prometheus.Histogram
	KnownIndices            map[string]bool
	BulkBuffer              []Metric
	BulkSize                uint
	Mux                     sync.Mutex
	MaxRetry                uint
	IndexName               string
}

type ElasticSearchClient interface {
	Perform(*http.Request) (*http.Response, error)
}

func NewBgMetadataElasticSearchConnector(elasticSearchClient ElasticSearchClient, registry prometheus.Registerer, bulkSize, maxRetry uint, indexName string) *BgMetadataElasticSearchConnector {
	var esc = BgMetadataElasticSearchConnector{
		client:     elasticSearchClient,
		BulkSize:   bulkSize,
		BulkBuffer: make([]Metric, 0, bulkSize),
		MaxRetry:   maxRetry,
		IndexName:  indexName,

		UpdatedMetrics: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "updated_metrics",
			Help:      "total number of metrics updated in ElasticSearch",
		}, []string{"status"}),

		HTTPErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_errors",
			Help:      "total number of http errors encountered partitionned by status code",
		}, []string{"code"}),

		WriteDurationMs: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "write_duration_ms",
			Help:      "time spent writing to ElasticSearch",
			Buckets:   []float64{250, 500, 750, 1000, 1500, 2000, 5000, 10000}}),
		DocumentBuildDurationMs: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "document_build_duration_ms",
			Help:      "time spent building an ElasticSearch document",
			Buckets:   []float64{1, 5, 10, 50, 100, 250, 500, 750, 1000, 2000}}),
	}
	_ = registry.Register(esc.UpdatedMetrics)
	_ = registry.Register(esc.WriteDurationMs)
	_ = registry.Register(esc.DocumentBuildDurationMs)
	if esc.IndexName == "" {
		esc.IndexName = default_metrics_metadata_index
	}

	esc.KnownIndices = map[string]bool{}
	return &esc
}

func CreateElasticSearchClient(server, username, password string) (*elasticsearch.Client, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{
			server,
		},
		Username: username,
		Password: password,
	}

	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("Error creating the ElasticSearch client: %s", err)
	}

	_, err = es.Info()
	if err != nil {
		log.Fatalf("Error getting ElasticSearch information response: %s", err)
	}

	return es, err
}

func NewBgMetadataElasticSearchConnectorWithDefaults(cfg *cfg.BgMetadataESConfig) *BgMetadataElasticSearchConnector {
	es, err := CreateElasticSearchClient(cfg.StorageServer, cfg.Username, cfg.Password)

	if err != nil {
		log.Fatalf("Could not create ElasticSearch connector: %w", err)
	}

	return NewBgMetadataElasticSearchConnector(es, prometheus.DefaultRegisterer, cfg.BulkSize, cfg.MaxRetry, cfg.IndexName)
}

func (esc *BgMetadataElasticSearchConnector) Close() {
}

func (esc *BgMetadataElasticSearchConnector) createIndexAndMapping(indexName string) error {
	indexCreateRequest := esapi.IndicesCreateRequest{Index: indexName}
	res, err := indexCreateRequest.Do(context.Background(), esc.client)

	// extract TODO error deserialize
	r := strings.NewReader(mapping)
	request := esapi.IndicesPutMappingRequest{Index: []string{indexName}, Body: r, DocumentType: documentType}
	res, err = request.Do(context.Background(), esc.client)

	if err != nil {
		return fmt.Errorf("Could not set ElasticSearch mapping: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		errorMessage, _ := ioutil.ReadAll(res.Body)
		return fmt.Errorf("Could not set ElasticSearch mapping (status %d, error: %s)", res.StatusCode, errorMessage)
	}

	return nil
}

// UpdateMetricMetadata stores the metric in a buffer, will bulkupdate when at full cap
// threadsafe
func (esc *BgMetadataElasticSearchConnector) UpdateMetricMetadata(metric Metric) error {
	esc.Mux.Lock()
	defer esc.Mux.Unlock()

	esc.BulkBuffer = append(esc.BulkBuffer, metric)
	if len(esc.BulkBuffer) == cap(esc.BulkBuffer) {
		esc.sendAndClearBuffer()
	}
	return nil
}

func (esc *BgMetadataElasticSearchConnector) sendAndClearBuffer() error {
	defer esc.clearBuffer()
	indexName, err := esc.getIndex()
	var errorMessage []byte
	var statusCode int

	if err != nil {
		esc.UpdatedMetrics.WithLabelValues("failure").Add(float64(len(esc.BulkBuffer)))
		return fmt.Errorf("Could not get index: %w", err)
	}

	for attempt := uint(0); attempt <= esc.MaxRetry; attempt++ {
		res, err := esc.bulkUpdate(indexName, esc.BulkBuffer)

		if err != nil {
			esc.UpdatedMetrics.WithLabelValues("failure").Add(float64(len(esc.BulkBuffer)))
			return fmt.Errorf("Could not write to index: %w", err)
		}

		if !res.IsError() {
			esc.UpdatedMetrics.WithLabelValues("success").Add(float64(len(esc.BulkBuffer)))
			res.Body.Close()
			return nil

		} else {
			esc.HTTPErrors.WithLabelValues(strconv.Itoa(res.StatusCode)).Inc()
			statusCode = res.StatusCode
			errorMessage, _ = ioutil.ReadAll(res.Body)
			res.Body.Close()
		}
	}

	esc.UpdatedMetrics.WithLabelValues("failure").Add(float64(len(esc.BulkBuffer)))
	return fmt.Errorf("Could not write to index (status %d, error: %s)", statusCode, errorMessage)

}

func (esc *BgMetadataElasticSearchConnector) clearBuffer() error {
	esc.BulkBuffer = esc.BulkBuffer[:0]
	return nil
}

func (esc *BgMetadataElasticSearchConnector) bulkUpdate(indexName string, metrics []Metric) (*esapi.Response, error) {
	timeBeforeBuild := time.Now()
	doc := BuildElasticSearchDocumentMulti(indexName, metrics)
	esc.DocumentBuildDurationMs.Observe(float64(time.Since(timeBeforeBuild).Milliseconds()))

	req := esapi.BulkRequest{
		Index:        indexName,
		Body:         strings.NewReader(doc),
		DocumentType: documentType,
	}

	timeBeforeWrite := time.Now()
	res, err := req.Do(context.Background(), esc.client)
	esc.WriteDurationMs.Observe(float64(time.Since(timeBeforeWrite).Milliseconds()))

	return res, err
}

func (esc *BgMetadataElasticSearchConnector) getIndex() (string, error) {
	indexName := esc.IndexName + time.Now().Format(metrics_metadata_index_suffix_format)

	_, isKnownIndex := esc.KnownIndices[indexName]
	if !isKnownIndex {
		err := esc.createIndexAndMapping(indexName)
		if err != nil {
			return "", err
		}
		esc.KnownIndices[indexName] = true
	}

	return indexName, nil
}

func (esc *BgMetadataElasticSearchConnector) InsertDirectory(dir MetricDirectory) error {
	return nil
}

func (esc *BgMetadataElasticSearchConnector) SelectDirectory(dir string) (string, error) {
	return "", nil
}
