package toki

import (
	"bytes"
	"fmt"
	"regexp"
	"unicode/utf8"
)

type Token uint32

const (
	EOF Token = 1<<32 - 1 - iota
	Error
)

type Position struct {
	Line   int
	Column int
}

func (this *Position) move(s []byte) {
	this.Line += bytes.Count(s, []byte{'\n'})
	last := bytes.LastIndex(s, []byte{'\n'})
	if last != -1 {
		this.Column = 1
	}
	this.Column += utf8.RuneCount(s[last+1:])
}

type Def struct {
	Token   Token
	Pattern string
	regexp  *regexp.Regexp
}

type Scanner struct {
	space *regexp.Regexp
	pos   Position
	input []byte
	def   []Def
}

type Result struct {
	Token Token
	Value []byte
	Pos   Position
}

func (this *Result) String() string {
	return fmt.Sprintf("Line: %d, Column: %d, %s", this.Pos.Line, this.Pos.Column, this.Value)
}

func NewScanner(def []Def) *Scanner {
	for i := range def {
		def[i].regexp = regexp.MustCompile("^" + def[i].Pattern)
	}
	return &Scanner{
		space: regexp.MustCompile(`^\s+`),
		def:   def,
	}
}

func (this *Scanner) SetInput(input string) {
	this.input = []byte(input)
	this.pos.Line = 1
	this.pos.Column = 1
}

func (this *Scanner) skip() {
	result := this.space.Find(this.input)
	if result == nil {
		return
	}
	this.pos.move(result)
	this.input = bytes.TrimPrefix(this.input, result)
}

func (this *Scanner) scan() *Result {
	this.skip()
	if len(this.input) == 0 {
		return &Result{Token: EOF, Pos: this.pos}
	}
	for _, r := range this.def {
		result := r.regexp.Find(this.input)
		if result == nil {
			continue
		}
		return &Result{Token: r.Token, Value: result, Pos: this.pos}
	}
	return &Result{Token: Error, Pos: this.pos}
}

func (this *Scanner) Peek() *Result {
	return this.scan()
}

func (this *Scanner) Next() *Result {
	t := this.scan()
	if t.Token == Error || t.Token == EOF {
		return t
	}
	this.input = bytes.TrimPrefix(this.input, t.Value)
	this.pos.move(t.Value)
	return t
}
