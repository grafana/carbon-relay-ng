package storage

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	lastComponent       = "__END__"
	componentsMaxLength = 64
	DirectorySeparator  = "."
)

type MetricDirectory struct {
	name       string
	components []string
	parent     string
}

func NewMetricDirectory(name string) *MetricDirectory {
	metricFields := strings.Split(name, ".")
	return &MetricDirectory{
		name:       name,
		components: append(metricFields, lastComponent),
		parent:     strings.Join(metricFields[:len(metricFields)-1], directorySeparator),
	}
}

// func (md *MetricDirectory) DoNothing() {}

func (md *MetricDirectory) generateParentDirectories() []*MetricDirectory {
	var path []string
	var parents []*MetricDirectory

	for _, component := range md.components[:len(md.components)-1] {
		path = append(path, component)
		parents = append(parents, NewMetricDirectory(strings.Join(path, DirectorySeparator)))
	}
	return parents
}

// UpdateDirectories is a recursive function that will test missing directories
// and insert them
func (md *MetricDirectory) UpdateDirectories(storageConnector BgMetadataStorageConnector) error {
	entry, err := storageConnector.SelectDirectory(md.name)
	if err != nil {
		if entry != "" {
			p := NewMetricDirectory(md.parent)
			p.UpdateDirectories(storageConnector)
			err = storageConnector.InsertDirectory(md)
			if err != nil {
				return fmt.Errorf("cannot insert metric directory: %s", err.Error())
			}
		}
	}
	return nil
}

// getUUID returns a hash of the name
func (md *MetricDirectory) getUUID() string {
	namespace, _ := uuid.Parse("00000000-1111-2222-3333-444444444444")
	sha := uuid.NewSHA1(namespace, []byte(md.name))
	return sha.String()
}

// ToESDocument returns a json string that represents the document
func (md *MetricDirectory) ToESDocument() string {
	var components = strings.Split(md.name, ".")
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`{"name":"%s",`, md.name))
	b.WriteString(fmt.Sprintf(`"parent":"%s",`, md.parent))
	b.WriteString(fmt.Sprintf(`"depth": "%d"`, len(components)-1))
	for i, component := range components {
		b.WriteString(",")
		b.WriteString(fmt.Sprintf(`"p%d":"%s"`, i, component))
	}

	b.WriteString(`}`)
	return b.String()

}
