package input

import "net"

func listen(addr string, handler Handler) (net.Listener, error) {
	laddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	l, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		return nil, err
	}
	log.Notice("listening on %v/tcp", laddr)
	go accept(l, handler)

	udp_addr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	udp_conn, err := net.ListenUDP("udp", udp_addr)
	if err != nil {
		return nil, err
	}
	log.Notice("listening on %v/udp", udp_addr)
	go handler.Handle(udp_conn)

	return l, nil
}

func accept(l *net.TCPListener, handler Handler) {
	for {
		c, err := l.AcceptTCP()
		if err != nil {
			log.Error(err.Error())
			break
		}
		go handler.Handle(c)
	}
}
