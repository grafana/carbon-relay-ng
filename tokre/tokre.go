// Package tokre tokenizes based on regular expressions.
package tokre

import (
	"fmt"
	"regexp"
	"strings"
)

type (
	// Classifies the type of token encountered.
	Token int

	// Defines a token type, with the Pattern being a regex.
	Def struct {
		Token   Token
		Pattern string
		regex   *regexp.Regexp
	}

	// The position within the input in characters.
	// Starting at (1,1). Note that multi-byte sequences must first
	// be converted to characters prior to use with this tokenizer.
	Position struct {
		Line   int
		Column int
	}

	// Created by NewScanner, this is the central place doing the tokenization.
	Scanner struct {
		defs     []Def
		input    string
		pos      Position
		space_re *regexp.Regexp
	}

	// The resulting tokens.
	// Each is from scanner.Next, and includes both the position and text found.
	Result struct {
		Token Token
		Pos   Position
		Value string
	}
)

// Represents the end of input.
const EOF Token = -1

// Represents some sort of error occurred, the input could not be matched to any token def.
const Error Token = -2

// Create a scanner tokenizer.
func NewScanner(defs []Def) *Scanner {
	for i := range defs {
		defs[i].regex = regexp.MustCompile("^" + defs[i].Pattern)
	}
	return &Scanner{
		defs:     defs,
		space_re: regexp.MustCompile(`^\s+`),
	}
}

// Start the tokenizer with an input string.
func (s *Scanner) SetInput(input string) {
	s.input = input
	s.pos = Position{1, 1}
}

// Get the next token from the input.
func (s *Scanner) Next() Result {
	s.skip_space()
	tok := s.match()
	s.consume(tok.Value)
	return tok
}

func (s *Scanner) Peek() Result {
	s.skip_space()
	return s.match()
}

// Pretty printable Result.
func (r Result) String() string {
	return fmt.Sprintf("(Ln %d, Col %d): %d \"%s\"", r.Pos.Line, r.Pos.Column, r.Token, r.Value)
}

// Consumes input, updating the state of the tokenizer.
func (s *Scanner) consume(text string) {
	s.pos.Line += strings.Count(text, "\n")
	last := strings.LastIndex(text, "\n")
	if last != -1 {
		s.pos.Column = 1
	}
	s.pos.Column += len(text) - last - 1

	s.input = strings.TrimPrefix(s.input, text)
}

// Skip spaces.
func (s *Scanner) skip_space() {
	result := s.space_re.FindString(s.input)
	if result == "" {
		return
	}
	s.consume(result)
}

// Match against the first Def that is at the head of the input.
func (s *Scanner) match() Result {
	if len(s.input) == 0 {
		return Result{Token: EOF, Pos: s.pos}
	}
	for _, r := range s.defs {
		result := r.regex.FindStringIndex(s.input)
		if result == nil {
			continue
		}
		return Result{Token: r.Token, Pos: s.pos, Value: s.input[result[0]:result[1]]}
	}
	return Result{Token: Error, Pos: s.pos}
}
