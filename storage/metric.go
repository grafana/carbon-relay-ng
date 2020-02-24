package storage

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"github.com/graphite-ng/carbon-relay-ng/encoding"
)

type Metric struct {
	name      string
	id        string
	createdOn gocql.UUID
	updatedOn gocql.UUID
	readOn    time.Time
	config    map[string]string
	tags      encoding.Tags
}

func NewMetric(name string, metadata MetricMetadata, tags encoding.Tags) *Metric {
	// TODO Add return error and handle it
	m := Metric{
		name:      name,
		createdOn: gocql.TimeUUID(),
		updatedOn: gocql.TimeUUID(),
		config:    metadata.Map(),
		tags:      tags,
	}
	m.id, _ = m.metricUUID()
	return &m
}

func (m *Metric) metricUUID() (string, error) {
	namespace, _ := uuid.Parse("00000000-1111-2222-3333-444444444444")
	name, _ := m.sanitizeMetricName()
	sha := uuid.NewSHA1(namespace, []byte(name))
	return sha.String(), nil
}

func (m *Metric) sanitizeMetricName() (string, error) {
	return strings.Replace(m.name, "..", ".", -1), nil
}

// ToESDocument returns a json string that represents the document
func (m *Metric) ToESDocument() string {
	var components = strings.Split(m.name, ".")
	var b strings.Builder

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000")
	configSerialized, _ := json.Marshal(m.config)

	b.WriteString(fmt.Sprintf(`{"name":"%s",`, m.name))
	b.WriteString(fmt.Sprintf(`"depth": "%d",`, len(components)-1))
	b.WriteString(fmt.Sprintf(`"uuid": "%s",`, m.id))
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
