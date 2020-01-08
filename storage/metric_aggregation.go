package storage

import (
	"regexp"

	"gopkg.in/ini.v1"
)

type StorageAggregation struct {
	ID                string
	pattern           string
	xFilesFactor      string
	aggregationMethod string
	patternRegex      *regexp.Regexp
}

func NewStorageAggregations(storageAggregationConf string) ([]StorageAggregation, error) {
	aggr := []StorageAggregation{}
	cfg, err := ini.Load(storageAggregationConf)
	if err != nil {
		return aggr, err
	}
	for _, section := range cfg.Sections()[1:] { // first element is empty default value
		pattern := section.KeysHash()["PATTERN"]
		patternRegex, _ := regexp.Compile(pattern)
		aggr = append(
			aggr,
			StorageAggregation{
				ID:                section.Name(),
				pattern:           pattern,
				xFilesFactor:      section.KeysHash()["XFILESFACTOR"],
				aggregationMethod: section.KeysHash()["AGGREGATIONMETHOD"],
				patternRegex:      patternRegex,
			},
		)
	}
	return aggr, nil
}
