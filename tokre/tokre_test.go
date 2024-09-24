package tokre

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	NUMBER Token = iota + 1
	DASH
	STRING
	EXCLAMATION
)

func TestToker(t *testing.T) {
	input := "3 -  2-1 - blast off!!!"

	scanner := NewScanner([]Def{
		{Token: NUMBER, Pattern: "[0-9]+"},
		{Token: DASH, Pattern: `-`},
		{Token: STRING, Pattern: "[a-z]+"},
		{Token: EXCLAMATION, Pattern: "!+"},
	})

	scanner.SetInput(input)

	expected := []Result{
		{NUMBER, Position{1, 1}, "3"}, {DASH, Position{1, 3}, "-"},
		{NUMBER, Position{1, 6}, "2"}, {DASH, Position{1, 7}, "-"},
		{NUMBER, Position{1, 8}, "1"}, {DASH, Position{1, 10}, "-"},
		{STRING, Position{1, 12}, "blast"}, {STRING, Position{1, 18}, "off"},
		{EXCLAMATION, Position{1, 21}, "!!!"}, {EOF, Position{1, 24}, ""},
	}
	for _, exp := range expected {
		token := scanner.Next()
		assert.Equal(t, token.Token, exp.Token)
		assert.Equal(t, token.Pos, exp.Pos)
		assert.Equal(t, token.Value, exp.Value)
		//t.Logf("%s", token)
	}
}
