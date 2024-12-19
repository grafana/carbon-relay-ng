// package whisper is a copy of some stuff we need from https://github.com/go-graphite/go-whisper
// in particular, it removes flock stuff which doesn't build on windows
package whisper

import (
	"fmt"
	"math"
	"strings"

	"github.com/raintank/dur"
)

const (
	PointSize = 12

	Seconds = 1
	Minutes = 60
	Hours   = 3600
	Days    = 86400
	Weeks   = 86400 * 7
	Years   = 86400 * 365
)

/*
Parse a retention definition as you would find in the storage-schemas.conf of a Carbon install.
Note that this only parses a single retention definition, if you have multiple definitions (separated by a comma)
you will have to split them yourself.

ParseRetentionDef("10s:14d") Retention{10, 120960}

See: http://graphite.readthedocs.org/en/1.0/config-carbon.html#storage-schemas-conf
*/
func ParseRetentionDef(retentionDef string) (*Retention, error) {
	parts := strings.Split(retentionDef, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("Not enough parts in retentionDef [%v]", retentionDef)
	}
	// raintank ParseDuration is a more flexible superset of graphite's
	// specification since many files use other units.
	precision, err := dur.ParseDuration(parts[0])
	if err != nil {
		return nil, fmt.Errorf("Failed to parse precision: %v", err)
	}

	points, err := dur.ParseDuration(parts[1])
	if err != nil {
		return nil, fmt.Errorf("Failed to parse points: %v", err)
	}
	points /= precision

	return &Retention{int(precision), int(points)}, err
}

func ParseRetentionDefs(retentionDefs string) (Retentions, error) {
	retentions := make(Retentions, 0)
	for _, retentionDef := range strings.Split(retentionDefs, ",") {
		retention, err := ParseRetentionDef(retentionDef)
		if err != nil {
			return nil, err
		}
		retentions = append(retentions, retention)
	}
	return retentions, nil
}

func validateRetentions(retentions Retentions) error {
	if len(retentions) == 0 {
		return fmt.Errorf("No retentions")
	}
	for i, retention := range retentions {
		if i == len(retentions)-1 {
			break
		}

		nextRetention := retentions[i+1]
		if !(retention.secondsPerPoint < nextRetention.secondsPerPoint) {
			return fmt.Errorf("A Whisper database may not be configured having two archives with the same precision (archive%v: %v, archive%v: %v)", i, retention, i+1, nextRetention)
		}

		if mod(nextRetention.secondsPerPoint, retention.secondsPerPoint) != 0 {
			return fmt.Errorf("Higher precision archives' precision must evenly divide all lower precision archives' precision (archive%v: %v, archive%v: %v)", i, retention.secondsPerPoint, i+1, nextRetention.secondsPerPoint)
		}

		if retention.MaxRetention() >= nextRetention.MaxRetention() {
			return fmt.Errorf("Lower precision archives must cover larger time intervals than higher precision archives (archive%v: %v seconds, archive%v: %v seconds)", i, retention.MaxRetention(), i+1, nextRetention.MaxRetention())
		}

		if retention.numberOfPoints < (nextRetention.secondsPerPoint / retention.secondsPerPoint) {
			return fmt.Errorf("Each archive must have at least enough points to consolidate to the next archive (archive%v consolidates %v of archive%v's points but it has only %v total points)", i+1, nextRetention.secondsPerPoint/retention.secondsPerPoint, i, retention.numberOfPoints)
		}
	}
	return nil
}

/*
A retention level.

Retention levels describe a given archive in the database. How detailed it is and how far back
it records.
*/
type Retention struct {
	secondsPerPoint int
	numberOfPoints  int
}

func (retention *Retention) MaxRetention() int {
	return retention.secondsPerPoint * retention.numberOfPoints
}

func (retention *Retention) Size() int {
	return retention.numberOfPoints * PointSize
}

func (retention *Retention) SecondsPerPoint() int {
	return retention.secondsPerPoint
}

func (retention *Retention) NumberOfPoints() int {
	return retention.numberOfPoints
}

func NewRetention(secondsPerPoint, numberOfPoints int) Retention {
	return Retention{
		secondsPerPoint,
		numberOfPoints,
	}
}

type Retentions []*Retention

func (r Retentions) Len() int {
	return len(r)
}

func (r Retentions) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

type retentionsByPrecision struct{ Retentions }

func (r retentionsByPrecision) Less(i, j int) bool {
	return r.Retentions[i].secondsPerPoint < r.Retentions[j].secondsPerPoint
}

/*
Implementation of modulo that works like Python
Thanks @timmow for this
*/
func mod(a, b int) int {
	return a - (b * int(math.Floor(float64(a)/float64(b))))
}
