package routing

import (
	"errors"
	"fmt"
	"github.com/Dieterbe/statsd-go"
	"log"
	"net"
	"regexp"
	"sync"
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
		//fmt.Println(route.Key + " route relay blocked looking for channel activity")
		select {
		case new_conn := <-conn_updates:
			conn = new_conn // can be nil and that's ok (it means we had to [re]connect but couldn't)
			if new_conn == nil {
				//fmt.Println(route.Key + " route relay -> got nil new conn")
			} else {
				//fmt.Println(route.Key + " route relay -> got a new conn!")
			}
			reconnecting = false
		case <-ticker.C:
			//fmt.Println(route.Key + " route relay -> assure conn")
			if conn != nil {
				//fmt.Println(route.Key + " connection already up and running")
				continue
			}
			if !reconnecting {
				//fmt.Println(route.Key + " route relay requesting new conn")
				reconnecting = true
				go new_conn()
			}
		case <-route.shutdown:
			//fmt.Println(route.Key + " route relay -> requested shutdown. quitting")
			return
		case buf := <-route.ch:
			//fmt.Println(route.Key + " route relay -> read buf")
			if conn == nil {
				route.instrument.Increment("route=" + route.Key + ".target_type=count.unit=Metric.direction=drop")
				fmt.Println(route.Key + " no conn, discarding")
				continue
			}
			//fmt.Println(route.Key + " writing to conn")
			route.instrument.Increment("route=" + route.Key + ".target_type=count.unit=Metric.direction=out")
			n, err := conn.Write(buf)
			if nil != err {
				route.instrument.Increment("route=" + route.Key + ".target_type=count.unit=Err")
				log.Println(err)
				conn.Close()
				conn = nil
				continue
			}
			//fmt.Println(route.Key + " check 1")
			if len(buf) != n {
				route.instrument.Increment("route=" + route.Key + ".target_type=count.unit=Err")
				log.Printf(route.Key+" truncated: %s\n", buf)
				conn.Close()
				conn = nil
			}
			////fmt.Println(route.Key + " check 2")
		}
	}

}

type Routes struct {
	Map  map[string]*Route
	lock sync.Mutex
}

func NewRoutes(routesMap map[string]*Route, instrument *statsd.Client) (routes Routes) {
	for k, _ := range routesMap {
		routesMap[k].instrument = instrument
		routesMap[k].Key = k
	}
	routes = Routes{Map: routesMap}
	return
}

// not thread safe, run this once only
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
	//fmt.Println("entering dispatch")
	routes.lock.Lock()
	defer routes.lock.Unlock()
	for _, route := range routes.Map {
		if route.Reg.Match(buf) {
			routed = true
			//fmt.Println("routing to " + k)
			route.ch <- buf
			if first_only {
				break
			}
		}
	}
	//fmt.Println("Dispatched")
	return routed
}

func (routes *Routes) List() map[string]Route {
	ret := make(map[string]Route)
	routes.lock.Lock()
	defer routes.lock.Unlock()
	for k, v := range routes.Map {
		ret[k] = Route{Key: v.Key, Patt: v.Patt, Addr: v.Addr}
		// notice: not set: Reg, ch, shutdown, instrument
	}
	return ret
}

func (routes *Routes) Add(key, patt, addr string, instrument *statsd.Client) error {
	routes.lock.Lock()
	defer routes.lock.Unlock()
	_, found := routes.Map[key]
	if found {
		return errors.New("route with given key already exists")
	}
	route := NewRoute(key, patt, addr, instrument)
	err := route.Compile() // later move this out of lock section
	if err != nil {
		return err
	}
	err = route.Run()
	if err != nil {
		return err
	}
	routes.Map[key] = route
	return nil
}

func (routes *Routes) Update(key string, addr, patt *string) error {
	routes.lock.Lock()
	defer routes.lock.Unlock()
	route, found := routes.Map[key]
	if !found {
		return errors.New("unknown route '" + key + "'")
	}
	if addr != nil {
		// TODO: clean way to switch remote endpoint, relay
	}
	if patt != nil {
		old_patt := route.Patt
		route.Patt = *patt
		err := route.Compile()
		if err != nil {
			route.Patt = old_patt
			route.Compile()
			return err
		}
	}
	return nil
}

func (routes *Routes) Del(key string) error {
	routes.lock.Lock()
	defer routes.lock.Unlock()
	route, found := routes.Map[key]
	if !found {
		return errors.New("unknown route '" + key + "'")
	}
	delete(routes.Map, key)
	err := route.Shutdown()
	if err != nil {
		// route removed from routing table but still trying to connect
		// it won't get new stuff on its input though
		return err
	}
	return nil
}
