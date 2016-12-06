package input

import (
	"net"
	"syscall"
	"time"

	"github.com/rcrowley/goagain"
)

func listen(addr string, handler Handler) (net.Listener, error) {
	// Follow the goagain protocol, <https://github.com/rcrowley/goagain>.
	l, ppid, err := goagain.GetEnvs()
	if err != nil {
		laddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return nil, err
		}
		l, err = net.ListenTCP("tcp", laddr)
		if err != nil {
			return nil, err
		}
		log.Notice("listening on %v/tcp", laddr)
		go accept(l.(*net.TCPListener), handler)
	} else {
		log.Notice("resuming listening on %v/tcp", l.Addr())
		go accept(l.(*net.TCPListener), handler)
		if err := goagain.KillParent(ppid); nil != err {
			return nil, err
		}
		for {
			err := syscall.Kill(ppid, 0)
			if err != nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

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
