package validate

import (
	"encoding/json"
	"fmt"
	m20 "github.com/metrics20/go-metrics20/carbon20"
)

type LevelLegacy struct {
	Level m20.ValidationLevelLegacy
}

func (m LevelLegacy) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Level.String())
}

func (l *LevelLegacy) UnmarshalText(text []byte) error {
	levels := map[string]m20.ValidationLevelLegacy{
		"strict": m20.StrictLegacy,
		"medium": m20.MediumLegacy,
		"none":   m20.NoneLegacy,
	}
	var err error
	var ok bool
	l.Level, ok = levels[string(text)]
	if !ok {
		err = fmt.Errorf("Invalid legacy validation level '%s'. Valid validation levels are 'strict', 'medium', and 'none'.", string(text))
	}
	return err
}

type LevelM20 struct {
	Level m20.ValidationLevelM20
}

func (m LevelM20) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Level.String())
}

func (l *LevelM20) UnmarshalText(text []byte) error {
	levels := map[string]m20.ValidationLevelM20{
		"medium": m20.MediumM20,
		"none":   m20.NoneM20,
	}
	var err error
	var ok bool
	l.Level, ok = levels[string(text)]
	if !ok {
		err = fmt.Errorf("Invalid M20 validation level '%s'. Valid validation levels are 'medium', and 'none'.", string(text))
	}
	return err
}
