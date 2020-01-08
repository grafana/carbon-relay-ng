package storage

import (
	"fmt"
	"regexp"

	"gopkg.in/ini.v1"
)

type StorageSchema struct {
	ID           string
	pattern      string
	retentions   string
	patternRegex *regexp.Regexp
}

func NewStorageSchemas(storageSchemasConf string) ([]StorageSchema, error) {
	schemas := []StorageSchema{}
	cfg, err := ini.Load(storageSchemasConf)
	if err != nil {
		return schemas, err
	}
	for _, section := range cfg.Sections()[1:] { // first element is empty default value
		pattern, ok := section.KeysHash()["PATTERN"]

		if ok != true {
			return schemas, fmt.Errorf("no pattern found in section %s in file %s", section.Name(), storageSchemasConf)
		}
		retentions, ok := section.KeysHash()["RETENTIONS"]

		if ok != true {
			return schemas, fmt.Errorf("no retention found in section %s in file %s", section.Name(), storageSchemasConf)
		}

		patternRegex, _ := regexp.Compile(pattern)
		schemas = append(
			schemas,
			StorageSchema{
				ID:           section.Name(),
				pattern:      pattern,
				retentions:   retentions,
				patternRegex: patternRegex,
			},
		)
	}
	return schemas, nil
}
