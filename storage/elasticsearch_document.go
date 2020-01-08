package storage

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

//BuildElasticSearchDocument returns a json string with the metric
func BuildElasticSearchDocument(metric Metric) string {
	var components = strings.Split(metric.name, ".") // TODO use slices to improve perf
	var b strings.Builder

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000")
	configSerialized, _ := json.Marshal(metric.config)

	b.WriteString(fmt.Sprintf(`{"name":"%s",`, metric.name))
	b.WriteString(fmt.Sprintf(`"depth": "%d",`, len(components)-1))
	b.WriteString(fmt.Sprintf(`"uuid": "%s",`, metric.id))
	b.WriteString(fmt.Sprintf(`"created_on":"%s",`, now))
	b.WriteString(fmt.Sprintf(`"updated_on":"%s",`, now))
	b.WriteString(fmt.Sprintf(`"read_on": null,`))
	b.WriteString(fmt.Sprintf(`"config": %s`, configSerialized))

	for i, component := range components {
		b.WriteString(",")
		b.WriteString(fmt.Sprintf(`"p%d":"%s"`, i, component))
	}

	b.WriteString(`}`)
	return b.String()
}

//BuildElasticSearchDocumentMulti returns a json string with the metric list
//to be bulk updated
func BuildElasticSearchDocumentMulti(indexName string, metrics []Metric) string {
	var b strings.Builder
	for _, metric := range metrics {
		b.WriteString(fmt.Sprintf(`{"index":{"_index":"%s","_id":"%s"}}`, indexName, metric.id))
		b.WriteString("\n")

		b.WriteString(BuildElasticSearchDocument(metric))
		b.WriteString("\n")

	}
	return b.String()
}
