package telnet

import (
	"fmt"
	"io"
	"net"
	"strings"

	"go.uber.org/zap"
)

var muxList []route

type adminFunc func(Req) error

type route struct {
	prefix string
	fn     adminFunc
}

type Req struct {
	Command []string
	Conn    *net.Conn // user api connection
}

func init() {
	muxList = make([]route, 0, 0)
}

func HandleFunc(prefix string, fn adminFunc) {
	muxList = append(muxList, route{prefix, fn})
}

func getHandler(cmd string) (fn adminFunc) {
	for _, route := range muxList {
		if strings.HasPrefix(cmd, route.prefix) {
			return route.fn
		}
	}
	return
}
func ListenAndServe(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer l.Close()
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		go handleApiRequest(conn)
	}
}

func handleApiRequest(conn net.Conn) {
	// Make a buffer to hold incoming data.
	buf := make([]byte, 1024)
	// Read the incoming connection into the buffer.
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				fmt.Println("read eof. closing")
			} else {
				fmt.Println("Error reading:", err.Error())
			}
			conn.Close()
			break
		}
		clean_cmd := strings.TrimSpace(string(buf[:n]))
		command := strings.Split(clean_cmd, " ")
		zap.S().Info("received command: '" + clean_cmd + "'")
		req := Req{command, &conn}
		fn := getHandler(clean_cmd)
		if fn != nil {
			err := fn(req)
			if err != nil {
				conn.Write([]byte(err.Error() + "\n"))
			}
		} else {
			conn.Write([]byte("unrecognized command\n"))
		}
	}
}
