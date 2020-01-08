package storage

// Needs defaults like this https://github.com/criteo/biggraphite/blob/master/biggraphite/drivers/cassandra.py#L232

import (
	"fmt"
	"strings"

	"github.com/gocql/gocql"
)

const (
	keyspace      = "biggraphite_metadata"
	metadataTable = "metrics_metadata"
	// rowSizePrecisionMs = 1000 * 1000

	// defaultKeyspace                       = "biggraphite"
	// defaultContactPoints                  = "127.0.0.1"
	// defaultPort                           = 9042
	// defaultTimeout                        = 10.0
	// defaultCompression                    = false
	// defaultMaxMetricsPerPattern           = 5000
	// defaultTrace                          = false
	// defaultBulkimport                     = false
	// defaultMaxQueriesPerPattern           = 42
	// defaultMaxConcurrentQueriesPerPattern = 4
	// defaultMaxConcurrentConnections       = 100
	// defaultMaxBatchUtil                   = 1000
	// defaultTimeoutQueryUtil               = 120
	// defaultReadSamplingRate               = 0.1
	// defaultUseLucene                      = false

	directorySeparator = DirectorySeparator

	// defaultMetaWriteConsistency  = "ONE"
	// defaultMetaSerialConsistency = "LOCAL_SERIAL"
	// defaultMetaReadConsistency   = "ONE"

	// defaultMetaBackgroundConsistency = "LOCAL_QUORUM"
)

type CassandraConnector struct {
	session *gocql.Session
}

func NewCassandraMetadata() *CassandraConnector {
	cluster := gocql.NewCluster("localhost")
	cluster.Keyspace = keyspace
	session, err := cluster.CreateSession()
	if err != nil {
		fmt.Println(err.Error())
		return &CassandraConnector{}
	}
	cc := CassandraConnector{
		session: session,
	}
	return &cc
}

func (cc *CassandraConnector) UpdateMetricMetadata(metric Metric) error {
	err := cc.session.Query(`UPDATE metrics_metadata SET id=?, config=?, updated_on=now() WHERE name=?`,
		metric.id, metric.config, metric.name).Exec()
	if err != nil {
		return err
	}
	return nil
}

func (cc *CassandraConnector) InsertDirectory(dir MetricDirectory) error {
	// TODO Use gocqlx and optimise query creation
	var queryArgs []string
	queryArgs = append(queryArgs, dir.name, dir.parent)

	columns := []string{"name", "parent"}
	for i, component := range dir.components {
		queryArgs = append(queryArgs, component)
		columns = append(columns, (fmt.Sprintf("component_%d", i)))
	}

	queryString := "INSERT INTO directories"
	queryString = queryString + "(" + strings.Join(columns, ", ") + ") VALUES ('" + strings.Join(queryArgs, "', '") + "')"

	query := cc.session.Query(queryString)
	if err := query.Exec(); err != nil {
		return fmt.Errorf("cannot insert directory into metadata: %s", err.Error())
	}
	return nil
}

func (cc *CassandraConnector) SelectDirectory(dir string) (string, error) {
	var directory string
	err := cc.session.Query(`SELECT name FROM directories WHERE name = ? LIMIT 1`,
		dir).Consistency(gocql.One).Scan(&directory)
	if err != nil {
		return directory, err
	}
	return directory, nil
}
