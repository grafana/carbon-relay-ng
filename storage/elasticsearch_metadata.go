package storage

import (
	"context"
	"encoding/json"
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
	"github.com/lestrrat-go/strftime"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	namespace                      = "elasticsearch"
	default_metrics_metadata_index = "biggraphite_metrics"
	default_index_date_format      = "%Y_%U"
	directories_index_suffix       = "directories"
	metricsMapping                 = `
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
	dirMapping = `
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
            "parent": {
                "type": "keyword"
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
	UpdatedDocuments        *prometheus.CounterVec
	HTTPErrors              *prometheus.CounterVec
	WriteDurationMs         prometheus.Histogram
	DocumentBuildDurationMs prometheus.Histogram
	KnownIndices            map[string]bool
	bulkBuffer              []ElasticSearchDocument
	BulkSize                uint
	mux                     sync.Mutex
	MaxRetry                uint
	IndexName, currentIndex string
	IndexDateFmt            string //strftime fmt string
	logger                  *zap.Logger
}

type ElasticSearchClient interface {
	Perform(*http.Request) (*http.Response, error)
}

// NewBgMetadataElasticSearchConnector : contructor for BgMetadataElasticSearchConnector
func newBgMetadataElasticSearchConnector(elasticSearchClient ElasticSearchClient, registry prometheus.Registerer, bulkSize, maxRetry uint, indexName, IndexDateFmt string) *BgMetadataElasticSearchConnector {
	var esc = BgMetadataElasticSearchConnector{
		client:       elasticSearchClient,
		BulkSize:     bulkSize,
		bulkBuffer:   make([]ElasticSearchDocument, 0, bulkSize),
		MaxRetry:     maxRetry,
		IndexName:    indexName,
		IndexDateFmt: IndexDateFmt,

		UpdatedDocuments: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "updated_documents",
			Help:      "total number of documents updated in ElasticSearch splited between metrics and directories",
		}, []string{"status", "type"}),

		HTTPErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_errors",
			Help:      "total number of http errors encountered partitionned by status code",
		}, []string{"code"}),

		WriteDurationMs: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "write_duration_ms",
			Help:      "time spent writing to ElasticSearch based on `took` field of response ",
			Buckets:   []float64{250, 500, 750, 1000, 1500, 2000, 5000, 10000}}),
		DocumentBuildDurationMs: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "document_build_duration_ms",
			Help:      "time spent building an ElasticSearch document",
			Buckets:   []float64{1, 5, 10, 50, 100, 250, 500, 750, 1000, 2000}}),
		logger: zap.L(),
	}
	_ = registry.Register(esc.UpdatedDocuments)
	_ = registry.Register(esc.WriteDurationMs)
	_ = registry.Register(esc.DocumentBuildDurationMs)
	if esc.IndexName == "" {
		esc.IndexName = default_metrics_metadata_index
	}
	if esc.IndexDateFmt == "" {
		esc.IndexDateFmt = default_index_date_format
	}

	esc.KnownIndices = map[string]bool{}
	return &esc
}

func createElasticSearchClient(server, username, password string) (*elasticsearch.Client, error) {
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

// NewBgMetadataElasticSearchConnectorWithDefaults is the public contructor of BgMetadataElasticSearchConnector
func NewBgMetadataElasticSearchConnectorWithDefaults(cfg *cfg.BgMetadataESConfig) *BgMetadataElasticSearchConnector {
	es, err := createElasticSearchClient(cfg.StorageServer, cfg.Username, cfg.Password)

	if err != nil {
		log.Fatalf("Could not create ElasticSearch connector: %w", err)
	}

	return newBgMetadataElasticSearchConnector(es, prometheus.DefaultRegisterer, cfg.BulkSize, cfg.MaxRetry, cfg.IndexName, cfg.IndexDateFmt)
}

func (esc *BgMetadataElasticSearchConnector) Close() {
}

func (esc *BgMetadataElasticSearchConnector) createIndicesAndMapping(metricIndexName, directoryIndexName string) error {
	indices := []struct{ name, mapping string }{{metricIndexName, metricsMapping}, {directoryIndexName, dirMapping}}
	for _, index := range indices {
		indexCreateRequest := esapi.IndicesCreateRequest{Index: index.name}
		res, err := indexCreateRequest.Do(context.Background(), esc.client)
		esc.logger.Info("using index", zap.String("name", index.name))
		// extract TODO error deserialize
		r := strings.NewReader(index.mapping)
		request := esapi.IndicesPutMappingRequest{Index: []string{index.name}, Body: r, DocumentType: documentType}
		res, err = request.Do(context.Background(), esc.client)

		if err != nil {
			return fmt.Errorf("Could not set ElasticSearch mapping: %w", err)
		}
		if res.StatusCode != http.StatusOK {
			errorMessage, _ := ioutil.ReadAll(res.Body)
			return fmt.Errorf("Could not set ElasticSearch mapping (status %d, error: %s)", res.StatusCode, errorMessage)
		}

	}
	return nil
}

// UpdateMetricMetadata stores the metric in a buffer, will bulkupdate when at full cap
// threadsafe
func (esc *BgMetadataElasticSearchConnector) UpdateMetricMetadata(metric *Metric) error {
	return esc.addDocumentToBuff(metric)
}

func (esc *BgMetadataElasticSearchConnector) addDocumentToBuff(doc ElasticSearchDocument) error {
	esc.mux.Lock()
	defer esc.mux.Unlock()

	esc.bulkBuffer = append(esc.bulkBuffer, doc)
	if len(esc.bulkBuffer) == cap(esc.bulkBuffer) {
		esc.sendAndClearBuffer()
	}
	return nil
}

func (esc *BgMetadataElasticSearchConnector) sendAndClearBuffer() error {
	defer esc.clearBuffer()
	metricIndex, directoryIndex, err := esc.getIndices()
	var errorMessage []byte
	var statusCode int

	if err != nil {
		esc.UpdatedDocuments.WithLabelValues("failure").Add(float64(len(esc.bulkBuffer)))
		return fmt.Errorf("Could not get index: %w", err)
	}

	timeBeforeBuild := time.Now()
	requestBody := BuildElasticSearchDocumentMulti(metricIndex, directoryIndex, esc.bulkBuffer)
	esc.DocumentBuildDurationMs.Observe(float64(time.Since(timeBeforeBuild).Milliseconds()))

	for attempt := uint(0); attempt <= esc.MaxRetry; attempt++ {
		res, err := esc.bulkUpdate(requestBody)

		if err != nil {
			// esapi resturns a nil body in case of error
			esc.UpdatedDocuments.WithLabelValues("failure", "any").Add(float64(len(esc.bulkBuffer)))
			return fmt.Errorf("Could not write to index: %w", err)
		}

		if !res.IsError() {
			esc.updateInternalMetrics(res)
			res.Body.Close()
			return nil

		} else {
			esc.HTTPErrors.WithLabelValues(strconv.Itoa(res.StatusCode)).Inc()
			statusCode = res.StatusCode
			errorMessage, _ = ioutil.ReadAll(res.Body)
			res.Body.Close()
		}
	}

	esc.UpdatedDocuments.WithLabelValues("failure", "any").Add(float64(len(esc.bulkBuffer)))
	return fmt.Errorf("Could not write to index (status %d, error: %s)", statusCode, errorMessage)

}

// updateInternalMetrics increments BGMetadataConnector's metrics,
func (esc *BgMetadataElasticSearchConnector) updateInternalMetrics(res *esapi.Response) {
	defer func() {
		if err := recover(); err != nil {
			esc.logger.Warn("malformed bulk response", zap.Error(err.(error)))
		}
	}()
	var mapResp map[string]interface{}
	json.NewDecoder(res.Body).Decode(&mapResp)
	esc.WriteDurationMs.Observe(mapResp["took"].(float64))
	for _, item := range mapResp["items"].([]interface{}) {
		mapCreate := item.(map[string]interface{})["create"].(map[string]interface{})
		// protected by esc.Mux currentIndex may not change while looping
		if int(mapCreate["status"].(float64)) == http.StatusCreated {
			if mapCreate["_index"] == esc.currentIndex {
				esc.UpdatedDocuments.WithLabelValues("created", "metric").Inc()
			} else {
				esc.UpdatedDocuments.WithLabelValues("created", "directory").Inc()
			}
		}
	}
}

func (esc *BgMetadataElasticSearchConnector) clearBuffer() error {
	esc.bulkBuffer = esc.bulkBuffer[:0]
	return nil
}

func (esc *BgMetadataElasticSearchConnector) bulkUpdate(body string) (*esapi.Response, error) {

	req := esapi.BulkRequest{
		Body:         strings.NewReader(body),
		DocumentType: documentType,
	}

	res, err := req.Do(context.Background(), esc.client)
	return res, err
}

func (esc *BgMetadataElasticSearchConnector) getIndices() (string, string, error) {
	metricIndexName, directoryIndexName := esc.getIndicesNames()
	esc.currentIndex = metricIndexName
	_, isKnownIndex := esc.KnownIndices[metricIndexName]

	// no need to test both
	if !isKnownIndex {
		err := esc.createIndicesAndMapping(metricIndexName, directoryIndexName)
		if err != nil {
			return "", "", err
		}
		esc.KnownIndices[metricIndexName] = true
	}

	return metricIndexName, directoryIndexName, nil
}

// InsertDirectory will add directory to the bulkBuffer
func (esc *BgMetadataElasticSearchConnector) InsertDirectory(dir *MetricDirectory) error {
	esc.addDocumentToBuff(dir)
	return nil
}

// SelectDirectory unused, no need in ES
// returns an error to signal that parent dir does not exist
func (esc *BgMetadataElasticSearchConnector) SelectDirectory(dir string) (string, error) {
	return dir, fmt.Errorf("")
}

func (esc *BgMetadataElasticSearchConnector) getIndicesNames() (metricIndexName, directoryIndexName string) {
	now := time.Now().UTC()
	date, err := strftime.Format(esc.IndexDateFmt, now)
	if err != nil {
		log.Fatalf("Index date format invalid strftime format: %s", err)
	}
	metricIndexName = fmt.Sprintf("%s_%s", esc.IndexName, date)
	directoryIndexName = fmt.Sprintf("%s_%s_%s", esc.IndexName, directories_index_suffix, date)
	return
}
