package routing

import (
	"github.com/Dieterbe/statsd-go"
	"errors"
	"fmt"
	"log"
	"net"
	"regexp"
	"time"
)

type Route struct {
	Key        string         // must be set on create. to identify in stats/logs
	Patt       string         // regex string. must be set on create
	Addr       string         // tcp dest. must be set on create
	Reg        *regexp.Regexp // compiled version of patt.  will be set on init
	ch         chan []byte    // to pump data to dest. will be ready on init
	shutdown   chan bool      // signals shutdown internally. will be set on init
	instrument *statsd.Client // to submit stats to.
}

// after creating, run Compile and Run!
func NewRoute(key, patt, addr string, instrument *statsd.Client) *Route {
	return &Route{key, patt, addr, nil, make(chan []byte), nil, instrument}
}

func (route *Route) Compile() error {
	regex, err := regexp.Compile(route.Patt)
	route.Reg = regex
	return err
}

func (route *Route) Run() error {
	route.ch = make(chan []byte)
	route.shutdown = make(chan bool)
	raddr, err := net.ResolveTCPAddr("tcp", route.Addr)
	if nil != err {
		return err
	}
	go route.relay(raddr)
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
func (route *Route) relay(raddr *net.TCPAddr) {
	laddr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0")
	period_assure_conn := time.Duration(60) * time.Second
	ticker := time.NewTicker(period_assure_conn)
	conn_updates := make(chan *net.TCPConn)
	var conn *net.TCPConn
	new_conn := func() {
		fmt.Printf("%v gonna open new conn to %v\n", route.Key, raddr)
		var err error
		fmt.Printf("%s pre dial ........  \n", route.Key)
		new_conn, err := net.DialTCP("tcp", laddr, raddr)
		fmt.Println(route.Key + "  ....dialed")
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
		fmt.Println(route.Key + " route relay blocked looking for channel activity")
		select {
		case new_conn := <-conn_updates:
			conn = new_conn // can be nil and that's ok (it means we had to [re]connect but couldn't)
			if new_conn == nil {
				fmt.Println(route.Key + " route relay -> got nil new conn")
			} else {
				fmt.Println(route.Key + " route relay -> got a new conn!")
			}
			reconnecting = false
		case <-ticker.C:
			fmt.Println(route.Key + " route relay -> assure conn")
			if conn != nil {
				fmt.Println(route.Key + " connection already up and running")
				continue
			}
			if !reconnecting {
				fmt.Println(route.Key + " route relay requesting new conn")
				reconnecting = true
				go new_conn()
			}
		case <-route.shutdown:
			fmt.Println(route.Key + " route relay -> requested shutdown. quitting")
			return
		case buf := <-route.ch:
			fmt.Println(route.Key + " route relay -> read buf")
			if conn == nil {
				route.instrument.Increment("route=" + route.Key + ".target_type=count.unit=Metric.direction=drop")
				fmt.Println(route.Key + " no conn, discarding")
				continue
			}
			fmt.Println(route.Key + " writing to conn")
			route.instrument.Increment("route=" + route.Key + ".target_type=count.unit=Metric.direction=out")
			n, err := conn.Write(buf)
			if nil != err {
				route.instrument.Increment("route=" + route.Key + ".target_type=count.unit=Err")
				log.Println(err)
				conn.Close()
				conn = nil
				continue
			}
			fmt.Println(route.Key + " check 1")
			if len(buf) != n {
				route.instrument.Increment("route=" + route.Key + ".target_type=count.unit=Err")
				log.Printf(route.Key+" truncated: %s\n", buf)
				conn.Close()
				conn = nil
			}
			fmt.Println(route.Key + " check 2")
		}
	}

}

// not safe for concurrency, do your own checks
type Routes struct {
	Map map[string]*Route
}

func NewRoutes(routesMap map[string]*Route, instrument *statsd.Client) (routes Routes) {
	for k, _ := range routesMap {
		routesMap[k].instrument = instrument
		routesMap[k].Key = k
	}
	routes = Routes{routesMap}
	return
}

func (routes *Routes) Init() error {
	for _, route := range routes.Map {
		err := route.Compile()
		if nil != err {
			return err
		}
		err = route.Run()
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
