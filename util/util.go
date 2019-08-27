package util

import "strings"

// some.host:2003 -> some_host_2003
// http://some.host:8080 -> http_some_host_8080
func AddrToPath(s string) string {
	s = strings.Replace(s, ".", "_", -1)
	s = strings.Replace(s, ":", "_", -1)
	return strings.Replace(s, "/", "", -1)
}

// some.host:2003, kafkaRoute -> kafkaRoute_some_host_2003
// http://some.host:8080, kafkaRoute -> kafkaRoute_http_some_host_8080
func Key(routeName, addr string) string {
	return routeName + "_" + AddrToPath(addr)
}

func MaxInt(x, y int) int {
	if x < y {
		return y
	}
	return x
}

func MinInt(x, y int) int {
	if x < y {
		return x
	}
	return y
}
