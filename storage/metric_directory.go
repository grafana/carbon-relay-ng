package storage

import (
	"fmt"
	"strings"
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

func NewMetricDirectory(name string) MetricDirectory {
	metricFields := strings.Split(name, ".")
	return MetricDirectory{
		name:       name,
		components: append(metricFields, lastComponent),
		parent:     strings.Join(metricFields[:len(metricFields)-1], directorySeparator) + ".",
	}
}

// func (md *MetricDirectory) DoNothing() {}

func (md *MetricDirectory) generateParentDirectories() []MetricDirectory {
	var path []string
	var parents []MetricDirectory

	for _, component := range md.components[:len(md.components)-1] {
		path = append(path, component)
		parents = append(parents, NewMetricDirectory(strings.Join(path, DirectorySeparator)))
	}
	return parents
}

func (md *MetricDirectory) UpdateDirectories(storageConnector BgMetadataStorageConnector) error {
	entry, err := storageConnector.SelectDirectory(md.name)
	if err != nil {
		if entry == "" {
			parentDirectories := md.generateParentDirectories()
			for _, pd := range parentDirectories {
				err = storageConnector.InsertDirectory(pd)
				if err != nil {
					fmt.Println(err.Error())
				}
			}
		} else {
			return fmt.Errorf("cannot check metric directory in cassandra: %s", err.Error())
		}
	}
	err = storageConnector.InsertDirectory(*md)
	if err != nil {
		fmt.Println(err.Error())
	}
	return nil
}
