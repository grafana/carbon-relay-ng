package main

type Table struct {
	sync.Mutex
	routes   []*Route
	spooldir string
	statsd   *statsD.Client
}

type Route struct {
	sync.Mutex
	Type  RouteType
	Dests []*Destination
}

type RouteType int
type sendAllMatch RouteType
type sendFirstMatch RouteType

func NewTable(spoolDir string, statsd *statsD.Client) *Table {
	routes := make([]*Route, 0, 0)
	return &Table{routes, spoolDir, statsd}
}

// not thread safe, run this once only
func (dests *Destinations) Run() error {
	for _, dest := range dests.Map {
		err := dest.Run()
		if err != nil {
			return err
		}
	}
	return nil
}
func (dests *Destinations) Dispatch(buf []byte, first_only bool) (destd bool) {
	//fmt.Println("entering dispatch")
	dests.Lock()
	defer dests.Unlock()
	for _, dest := range dests.Map {
		if dest.Reg.Match(buf) {
			destd = true
			//fmt.Println("routing to " + dest.Key)
			dest.ch <- buf
			if first_only {
				break
			}
		}
	}
	//fmt.Println("Dispatched")
	return destd
}

func (dests *Destinations) List() map[string]Destination {
	ret := make(map[string]Destination)
	dests.Lock()
	defer dests.Unlock()
	for k, v := range dests.Map {
		ret[k] = *v.Copy()
	}
	return ret
}

func (dests *Destinations) Add(key, patt, addr string, spool, pickle bool, statsd *statsD.Client) error {
	dests.Lock()
	defer dests.Unlock()
	_, found := dests.Map[key]
	if found {
		return errors.New("dest with given key already exists")
	}
	dest, err := NewDestination(key, patt, addr, dests.SpoolDir, spool, pickle, statsd)
	if err != nil {
		return err
	}
	err = dest.Run()
	if err != nil {
		return err
	}
	dests.Map[key] = dest
	return nil
}

func (dests *Destinations) Update(key string, addr, patt *string) error {
	dests.Lock()
	defer dests.Unlock()
	dest, found := dests.Map[key]
	if !found {
		return errors.New("unknown dest '" + key + "'")
	}
	if patt != nil {
		err := dest.updatePattern(*patt)
		if err != nil {
			return err
		}
	}
	if addr != nil {
		return dest.updateConn(*addr)
	}
	return nil
}

func (dests *Destinations) Del(key string) error {
	dests.Lock()
	defer dests.Unlock()
	dest, found := dests.Map[key]
	if !found {
		return errors.New("unknown dest '" + key + "'")
	}
	delete(dests.Map, key)
	err := dest.Shutdown()
	if err != nil {
		// dest removed from routing table but still trying to connect
		// it won't get new stuff on its input though
		return err
	}
	return nil
}
