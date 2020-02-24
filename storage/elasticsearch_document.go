package storage

import (
	"fmt"
	"strings"
)

//ElasticSearchDocument interface for metrics and directories
type ElasticSearchDocument interface {
	ToESDocument() string
}

//BuildElasticSearchDocumentMulti returns a json string with the metric list
//to be bulk updated
func BuildElasticSearchDocumentMulti(MetricIndex, DirectoryIndex string, docs []ElasticSearchDocument) string {
	var b strings.Builder
	var currentIndex, currentId string
	for _, doc := range docs {
		switch typedDoc := doc.(type) {
		case *Metric:
			currentIndex = MetricIndex
			currentId = typedDoc.id
		case *MetricDirectory:
			currentIndex = DirectoryIndex
			currentId = typedDoc.getUUID()
		}
		b.WriteString(fmt.Sprintf(`{"create":{"_index":"%s","_id":"%s"}}`, currentIndex, currentId))
		b.WriteString("\n")
		b.WriteString(doc.ToESDocument())
		b.WriteString("\n")

	}
	return b.String()
}
