package mdns

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
)

// Responder is a deterministic A-record responder used only by the loopback
// simulator acceptance programs.
type Responder struct {
	conns    []*net.UDPConn
	host     string
	response []byte
	once     sync.Once
}

// NewResponder joins the IPv4 mDNS group on every usable interface and
// prepares one synthetic A record.
func NewResponder(host string, ip net.IP) (*Responder, error) {
	if !strings.HasSuffix(strings.TrimSuffix(strings.ToLower(host), "."), ".local") {
		return nil, errors.New("simulator mDNS name must end in .local")
	}
	message, err := response(host, ip)
	if err != nil {
		return nil, err
	}
	interfaces, err := multicastInterfaces()
	if err != nil {
		return nil, errors.New("could not list simulator mDNS interfaces")
	}
	conns := make([]*net.UDPConn, 0, len(interfaces))
	for i := range interfaces {
		conn, listenErr := net.ListenMulticastUDP("udp4", &interfaces[i], multicastAddress)
		if listenErr != nil {
			continue
		}
		_ = conn.SetReadBuffer(maxDNSMessageSize)
		conns = append(conns, conn)
	}
	if len(conns) == 0 {
		return nil, errors.New("could not open simulator mDNS listener")
	}
	return &Responder{conns: conns, host: canonicalName(host), response: message}, nil
}

// Serve answers matching A or ANY questions until ctx is canceled or Close is
// called. Malformed and unrelated multicast traffic is ignored.
func (r *Responder) Serve(ctx context.Context) error {
	stop := context.AfterFunc(ctx, func() { _ = r.Close() })
	defer stop()
	results := make(chan error, len(r.conns))
	for _, conn := range r.conns {
		go func(conn *net.UDPConn) {
			results <- r.serveConn(ctx, conn)
		}(conn)
	}
	var serveErrors []error
	for range r.conns {
		if err := <-results; err != nil {
			serveErrors = append(serveErrors, err)
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	return errors.Join(serveErrors...)
}

func (r *Responder) serveConn(ctx context.Context, conn *net.UDPConn) error {
	buffer := make([]byte, maxDNSMessageSize)
	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		items, err := questions(buffer[:n])
		if err != nil {
			continue
		}
		for _, item := range items {
			if strings.EqualFold(item.name, r.host) && item.qclass&classMask == classIN && (item.qtype == typeA || item.qtype == typeANY) {
				if _, err := conn.WriteToUDP(r.response, multicastAddress); err != nil && ctx.Err() == nil {
					return err
				}
				break
			}
		}
	}
}

// Close stops the responder. It is safe to call more than once.
func (r *Responder) Close() error {
	var err error
	r.once.Do(func() {
		closeErrors := make([]error, 0, len(r.conns))
		for _, conn := range r.conns {
			if closeErr := conn.Close(); closeErr != nil {
				closeErrors = append(closeErrors, closeErr)
			}
		}
		err = errors.Join(closeErrors...)
	})
	return err
}
