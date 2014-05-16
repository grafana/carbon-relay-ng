package routes

type Route struct {
	Patt string        // regex string. must be set on create
	Addr string        // tcp dest. must be set on create
	Reg  regexp.Regexp // compiled version of patt.  will be set on init
	ch   chan []byte   // to pump data to dest. will be ready on init
}

// after creating, run Compile and Run!
func NewRoute(patt, addr string) *Route {
	return &Route{patt, addr}
}

func (route *Route) Compile() err {
	route.Reg, err = regexp.Compile(route.Patt)
	return
}

func (route *Route) Run() err {
	route.ch = make(chan []byte)
	raddr, err := net.ResolveTCPAddr("tcp", route.addr)
	if nil != err {
		return err
	}
	go route.relay(raddr)
}

// not safe for concurrency, do your own checks
type Routes struct {
	Map map[string]*Route
}

func (routes *Routes) Init() err {
	for key, route := range routes.Map {
		err := route.Compile()
		if nil != err {
			return err
		}
		err := route.Run()
		if nil != err {
			return err
		}
	}
}

func (route *Route) relay(raddr *net.TCPAddr) {
	var buf []byte
	laddr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0")
	for {
		c, err := net.DialTCP("tcp", laddr, raddr)
		if nil != err {
			log.Println(err)
			time.Sleep(10e9) // this sleep would block channel reads
		}
		// TODO c.SetTimeout(60e9)
		if nil != buf && c != nil && !write(buf, c) {
			continue // TODO Decide when to drop this buffer and move on.
		}
		for {
			buf := <-route.ch
			if !write(buf, c) {
				break
			}
		}
	}
}

func write(buf []byte, c *net.TCPConn) bool {
	n, err := c.Write(buf)
	if nil != err {
		log.Println(err)
		c.Close()
		return false
	}
	if len(buf) != n {
		log.Printf("truncated: %s\n", buf)
		c.Close()
		return false
	}
	return true
}
