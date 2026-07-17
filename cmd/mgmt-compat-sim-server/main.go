// Command mgmt-compat-sim-server exposes deterministic compatibility scenarios
// for real MGMT process tests. The Native API peer remains loopback-only. An
// optional non-loopback TCP forwarder is allowed only in a private Linux
// network namespace for an unchanged MCL file that contains a documentation IP.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/flavio-fernandes/go-aioesphomeapi/internal/mdns"
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
	"google.golang.org/protobuf/proto"
)

func main() {
	scenarioName := flag.String("scenario", "", "scenario: basic-io or blink")
	address := flag.String("listen", "127.0.0.1:6053", "loopback Native API address")
	forwardAddress := flag.String("forward-listen", "", "optional private-namespace TCP forward address")
	parentNetworkNamespace := flag.String("parent-netns", "", "parent network namespace identifier required by non-loopback forwarding")
	mdnsHost := flag.String("mdns-host", "", "optional synthetic .local name for loopback acceptance")
	flag.Parse()

	scenario, err := namedScenario(*scenarioName)
	if err != nil {
		log.Fatal(err)
	}
	if *forwardAddress != "" && !forwardAllowed(*forwardAddress, networkNamespaceIsPrivate(*parentNetworkNamespace)) {
		log.Fatal("non-loopback forwarding requires a private Linux network namespace")
	}

	listener, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatal("start simulator listener: ", err)
	}
	device := simulator.New(scenario)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveDone := make(chan error, 1)
	go func() { serveDone <- device.Serve(listener) }()
	go printCommands(device.Commands())
	var mdnsResponder *mdns.Responder
	if *mdnsHost != "" {
		host, _, splitErr := net.SplitHostPort(listener.Addr().String())
		if splitErr != nil {
			_ = device.Close()
			log.Fatal("read simulator listener address: ", splitErr)
		}
		mdnsResponder, err = mdns.NewResponder(*mdnsHost, net.ParseIP(host))
		if err != nil {
			_ = device.Close()
			log.Fatal("start simulator mDNS responder: ", err)
		}
		go func() {
			if serveErr := mdnsResponder.Serve(ctx); serveErr != nil {
				log.Print("simulator mDNS responder: ", serveErr)
			}
		}()
	}

	var forwarder *tcpForwarder
	if *forwardAddress != "" {
		forwarder, err = newTCPForwarder(*forwardAddress, listener.Addr().String())
		if err != nil {
			_ = device.Close()
			log.Fatal("start private-namespace forwarder: ", err)
		}
		go forwarder.serve()
	}

	fmt.Printf("secure %s simulator ready\n", *scenarioName)

	select {
	case <-ctx.Done():
	case err := <-serveDone:
		if err != nil {
			log.Print(err)
		}
	}
	if forwarder != nil {
		_ = forwarder.Close()
	}
	if mdnsResponder != nil {
		_ = mdnsResponder.Close()
	}
	_ = device.Close()
}

func namedScenario(name string) (simulator.Scenario, error) {
	switch name {
	case "basic-io":
		return simulator.BasicIOScenario(), nil
	case "blink":
		return simulator.BlinkScenario(), nil
	default:
		return simulator.Scenario{}, fmt.Errorf("unknown simulator scenario %q", name)
	}
}

func forwardAllowed(address string, privateNamespace bool) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if host == "localhost" || (ip != nil && ip.IsLoopback()) {
		return true
	}
	return privateNamespace
}

func networkNamespaceIsPrivate(parent string) bool {
	self, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		return false
	}
	return namespaceIsPrivate(self, parent)
}

func namespaceIsPrivate(self, init string) bool {
	return strings.HasPrefix(self, "net:[") && strings.HasPrefix(init, "net:[") && self != init
}

type tcpForwarder struct {
	listener net.Listener
	target   string
	done     chan struct{}
	once     sync.Once
}

func newTCPForwarder(address, target string) (*tcpForwarder, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	return &tcpForwarder{listener: listener, target: target, done: make(chan struct{})}, nil
}

func (f *tcpForwarder) Close() error {
	var err error
	f.once.Do(func() {
		close(f.done)
		err = f.listener.Close()
	})
	return err
}

func (f *tcpForwarder) serve() {
	for {
		front, err := f.listener.Accept()
		if err != nil {
			return
		}
		go f.forward(front)
	}
}

func (f *tcpForwarder) forward(front net.Conn) {
	defer front.Close()
	back, err := net.DialTimeout("tcp", f.target, time.Second)
	if err != nil {
		return
	}
	defer back.Close()

	completed := make(chan struct{}, 2)
	copyOneWay := func(dst, src net.Conn) {
		_, _ = io.Copy(dst, src)
		completed <- struct{}{}
	}
	go copyOneWay(back, front)
	go copyOneWay(front, back)
	select {
	case <-completed:
	case <-f.done:
	}
}

func printCommands(commands <-chan proto.Message) {
	for command := range commands {
		switch value := command.(type) {
		case *pb.SwitchCommandRequest:
			fmt.Printf("received switch command: key=%d state=%t\n", value.Key, value.State)
		case *pb.NumberCommandRequest:
			fmt.Printf("received number command: key=%d value=%g\n", value.Key, value.State)
		case *pb.ButtonCommandRequest:
			fmt.Printf("received button command: key=%d\n", value.Key)
		}
	}
}
