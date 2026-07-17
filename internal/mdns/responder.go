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
	conn     *net.UDPConn
	host     string
	response []byte
	once     sync.Once
}

// NewResponder joins the IPv4 mDNS group and prepares one synthetic A record.
func NewResponder(host string, ip net.IP) (*Responder, error) {
	if !strings.HasSuffix(strings.TrimSuffix(strings.ToLower(host), "."), ".local") {
		return nil, errors.New("simulator mDNS name must end in .local")
	}
	message, err := response(host, ip)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenMulticastUDP("udp4", nil, multicastAddress)
	if err != nil {
		return nil, errors.New("could not open simulator mDNS listener")
	}
	_ = conn.SetReadBuffer(maxDNSMessageSize)
	return &Responder{conn: conn, host: canonicalName(host), response: message}, nil
}

// Serve answers matching A or ANY questions until ctx is canceled or Close is
// called. Malformed and unrelated multicast traffic is ignored.
func (r *Responder) Serve(ctx context.Context) error {
	stop := context.AfterFunc(ctx, func() { _ = r.Close() })
	defer stop()
	buffer := make([]byte, maxDNSMessageSize)
	for {
		n, _, err := r.conn.ReadFromUDP(buffer)
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
				if _, err := r.conn.WriteToUDP(r.response, multicastAddress); err != nil && ctx.Err() == nil {
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
	r.once.Do(func() { err = r.conn.Close() })
	return err
}
