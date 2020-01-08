package storage

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type MetricMetadata struct {
	aggregator         string
	carbonXfilesfactor string
	retention          string
}

type stage struct {
	precision time.Duration
	duration  time.Duration
	points    int64
}

func NewStage(precision, duration string) (stage, error) {
	s := stage{}
	var err error
	s.precision, err = time.ParseDuration(precision)
	if err != nil {
		s.precision, err = dayOrYearToDuration(precision)
		if err != nil {
			return s, err
		}
	}
	s.duration, err = time.ParseDuration(duration)
	if err != nil {
		s.duration, err = dayOrYearToDuration(duration)
		if err != nil {
			return s, err
		}
	}
	s.points = int64(s.duration / s.precision)
	return s, nil
}

func (s *stage) String() string {
	return fmt.Sprintf("%d*%ds", s.points, int64(s.precision.Seconds()))
}

func NewMetricMetadata(name string, schemas []StorageSchema, aggregations []StorageAggregation) MetricMetadata {
	const (
		defaultAggregator   = "average"
		defaultXFilesFactor = "0.5"
		defaultRetention    = "60s:8d,1h:30d,1d:2y"
	)
	aggr, schema := matchMetric(name, aggregations, schemas)
	mm := MetricMetadata{
		aggregator:         aggr.aggregationMethod,
		carbonXfilesfactor: aggr.xFilesFactor,
		retention:          parseStages(schema.retentions),
	}
	if mm.aggregator == "" {
		mm.aggregator = defaultAggregator
	}
	if mm.carbonXfilesfactor == "" {
		mm.carbonXfilesfactor = defaultXFilesFactor
	}
	if mm.retention == "" {
		mm.retention = parseStages(defaultRetention)
	}
	return mm
}

// time.ParseDuration doesn't handle days and years
func dayOrYearToDuration(t string) (time.Duration, error) {
	value, err := strconv.ParseInt(t[:len(t)-1], 10, 64)
	if err != nil {
		return 0, err
	}
	unit := t[len(t)-1:]
	switch unit {
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "y":
		return time.Duration(value) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("cannot convert '%s' to duration", t)
	}
}

func stagesFromGroups(stageGroups string) ([]stage, error) {
	// expect value in this format 60s:8d,1h:30d,1d:2y
	rawStages := strings.Split(stageGroups, ",")
	stages := []stage{}
	for _, rs := range rawStages {
		stageData := strings.Split(rs, ":")
		// handle errors
		newStage, err := NewStage(stageData[0], stageData[1])
		if err != nil {
			return stages, fmt.Errorf("Failed to create new stage")
		}
		stages = append(stages, newStage)
	}
	return stages, nil
}

func matchMetric(name string, aggrs []StorageAggregation, schemas []StorageSchema) (StorageAggregation, StorageSchema) {
	matchedAg := StorageAggregation{}
	matchedSc := StorageSchema{}
	for _, ag := range aggrs {
		// matched, err := regexp.MatchString(ag.pattern, name)
		matched := ag.patternRegex.MatchString(name)
		if matched {
			matchedAg = ag
			// stop on first match
			break
		}
	}
	for _, sc := range schemas {
		// matched, err := regexp.MatchString(sc.pattern, name)
		matched := sc.patternRegex.MatchString(name)
		if matched {
			matchedSc = sc
			// stop on first match
			break
		}
	}
	return matchedAg, matchedSc
}

func parseStages(stageGroups string) string {
	stages, err := stagesFromGroups(stageGroups)
	if err != nil {
		return ""
	}
	var parsedStages []string
	for _, s := range stages {
		parsedStages = append(parsedStages, s.String())
	}
	return strings.Join(parsedStages, ":")
}

func (mm *MetricMetadata) Map() map[string]string {
	config := map[string]string{
		"aggregator":          mm.aggregator,
		"carbon_xfilesfactor": mm.carbonXfilesfactor,
		"retention":           mm.retention,
	}
	return config
}
