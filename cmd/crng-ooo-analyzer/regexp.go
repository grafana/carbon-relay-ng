package main

import "regexp/syntax"

type group struct {
	Num  int
	Name string
	Patt string
}

// groups returns the matching groups from a parsed regex
func groups(re *syntax.Regexp) []group {
	var g []group
	for _, s := range re.Sub {
		if s.Op == syntax.OpCapture {
			g = append(g, group{
				Num:  s.Cap,
				Name: s.Name,
				Patt: s.Sub0[0].String(),
			})
		}
	}
	return g
}
