package main

// expvar doesn't allow reusing the same variables, but we need this.
// let's say we reconnect to a tcp endpoint we connected to earlier, then we want
// to increment that counter again

import "expvar"

// this is technically racey, but we know that we'll never create two ints so quickly in a row
// so this is never a real problem.
func Int(key string) *expvar.Int {
	v := expvar.Get(key)
	if v != nil {
		return v.(*expvar.Int)
	}
	n := new(expvar.Int)
	expvar.Publish(key, n)
	return n
}
