package util

import "strings"

// some.host:2003 -> some_host_2003
// http://some.host:8080 -> http_some_host_8080
func AddrToPath(s string) string {
	s = strings.Replace(s, ".", "_", -1)
	s = strings.Replace(s, ":", "_", -1)
	return strings.Replace(s, "/", "", -1)
}
