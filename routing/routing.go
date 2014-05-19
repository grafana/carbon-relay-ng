package routing

import (
	"errors"
	"fmt"
	"log"
	"net"
	"regexp"
	"time"
)

type Route struct {
	Patt     string         // regex string. must be set on create
	Addr     string         // tcp dest. must be set on create
	Reg      *regexp.Regexp // compiled version of patt.  will be set on init
	ch       chan []byte    // to pump data to dest. will be ready on init
	shutdown chan bool      // signals shutdown internally. will be set on init
}

// after creating, run Compile and Run!
func NewRoute(patt, addr string) *Route {
	return &Route{patt, addr, nil, make(chan []byte), nil}
}

func (route *Route) Compile() error {
	regex, err := regexp.Compile(route.Patt)
	route.Reg = regex
	return err
}

func (route *Route) Run(tmp string) error {
	route.ch = make(chan []byte)
	route.shutdown = make(chan bool)
	raddr, err := net.ResolveTCPAddr("tcp", route.Addr)
	if nil != err {
		return err
	}
	go route.relay(raddr, tmp)
	return err
}

func (route *Route) Shutdown() error {
	if route.shutdown == nil {
		return errors.New("not running yet")
	}
	route.shutdown <- true
	return nil
}

// TODO func (l *TCPListener) SetDeadline(t time.Time)
// TODO Decide when to drop this buffer and move on.
// currently, we drop packets while we set up connection
// later we might want to queue up
func (route *Route) relay(raddr *net.TCPAddr, tmp string) {
	laddr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0")
	period_assure_conn := time.Duration(60) * time.Second
	ticker := time.NewTicker(period_assure_conn)
	conn_updates := make(chan *net.TCPConn)
	var conn *net.TCPConn
	new_conn := func() {
		fmt.Printf("%v gonna open new conn to %v\n", tmp, raddr)
		var err error
		fmt.Printf("%s pre dial ........  \n", tmp)
		new_conn, err := net.DialTCP("tcp", laddr, raddr)
		fmt.Println(tmp + "  ....dialed")
		if nil != err {
			log.Println(err)
			conn_updates <- nil
			return
		}
		conn_updates <- new_conn
	}
	reconnecting := true
	go new_conn()
	for {
		fmt.Println(tmp + " route relay blocked looking for channel activity")
		select {
		case new_conn := <-conn_updates:
			conn = new_conn // can be nil and that's ok (it means we had to [re]connect but couldn't)
			if new_conn == nil {
				fmt.Println(tmp + " route relay -> got nil new conn")
			} else {
				fmt.Println(tmp + " route relay -> got a new conn!")
			}
			reconnecting = false
		case <-ticker.C:
			fmt.Println(tmp + " route relay -> assure conn")
			if conn != nil {
				fmt.Println(tmp + " connection already up and running")
				continue
			}
			if !reconnecting {
				fmt.Println(tmp + " route relay requesting new conn")
				reconnecting = true
				go new_conn()
			}
		case <-route.shutdown:
			fmt.Println(tmp + " route relay -> requested shutdown. quitting")
			return
		case buf := <-route.ch:
			fmt.Println(tmp + " route relay -> read buf")
			if conn == nil {
				// TODO statsd calls
				fmt.Println(tmp + " no conn, discarding")
				continue
			}
			fmt.Println(tmp + " writing to conn")
			n, err := conn.Write(buf)
			if nil != err {
				log.Println(err)
				conn.Close()
				conn = nil
				continue
			}
			fmt.Println(tmp + " check 1")
			if len(buf) != n {
				log.Printf(tmp+" truncated: %s\n", buf)
				conn.Close()
				conn = nil
			}
			fmt.Println(tmp + " check 2")
		}
	}

}

// not safe for concurrency, do your own checks
type Routes struct {
	Map map[string]*Route
}

func (routes *Routes) Init() error {
	for k, route := range routes.Map {
		err := route.Compile()
		if nil != err {
			return err
		}
		err = route.Run(k)
		if nil != err {
			return err
		}
	}
	return nil
}
func (routes *Routes) Dispatch(buf []byte, first_only bool) (routed bool) {
	fmt.Println("entering dispatch")
	for k, route := range routes.Map {
		if route.Reg.Match(buf) {
			routed = true
			fmt.Println("routing to " + k)
			route.ch <- buf
			if first_only {
				break
			}
		}
	}
	fmt.Println("Dispatched")
	return routed
}
