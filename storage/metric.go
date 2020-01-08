package storage

import (
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

func NewMetric(name string, metadata MetricMetadata, tags encoding.Tags) Metric {
	// TODO Add return error and handle it
	m := Metric{
		name:      name,
		createdOn: gocql.TimeUUID(),
		updatedOn: gocql.TimeUUID(),
		config:    metadata.Map(),
		tags:      tags,
	}
	m.id, _ = m.metricUUID()
	return m
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
