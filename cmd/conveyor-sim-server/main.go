// Command conveyor-sim-server exposes the conveyor simulator on loopback for
// external-consumer tests such as MGMT acceptance. It never accepts a wildcard
// or non-loopback listener.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flavio-fernandes/go-aioesphomeapi/internal/mdns"
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
	"google.golang.org/protobuf/proto"
)

func main() {
	address := flag.String("listen", "127.0.0.1:6053", "loopback TCP address")
	mdnsHost := flag.String("mdns-host", "", "optional synthetic .local name for isolated acceptance")
	cycles := flag.Int("cycles", 0, "bricks to run through the simulated belt: 0 for a static device that only reports its boot state, negative to keep going until interrupted")
	travel := flag.Duration("travel", 1200*time.Millisecond, "time a brick takes to travel from the entry sensor to the exit sensor")
	dwell := flag.Duration("dwell", 1500*time.Millisecond, "time a measured brick is left at the exit before the simulated operator lifts it off")
	flag.Parse()

	listener, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatal("start simulator listener: ", err)
	}
	device := simulator.New(simulator.ConveyorScenario())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var mdnsResponder *mdns.Responder
	if *mdnsHost != "" {
		mdnsResponder, err = mdns.NewResponder(*mdnsHost, net.IPv4(127, 0, 0, 1))
		if err != nil {
			_ = device.Close()
			log.Fatal("start simulator mDNS responder: ", err)
		}
		defer mdnsResponder.Close()
		go func() {
			if serveErr := mdnsResponder.Serve(ctx); serveErr != nil {
				log.Print("simulator mDNS responder: ", serveErr)
			}
		}()
	}
	go func() {
		<-ctx.Done()
		_ = device.Close()
	}()
	// With cycles requested the firmware model consumes the command stream, so
	// it can react to the motor and print each command exactly once. A negative
	// count runs bricks through until interrupted.
	if *cycles != 0 {
		firmware := newConveyorFirmware(device, *cycles, *travel, *dwell)
		go firmware.run(device.Commands())
	} else {
		go printCommands(device.Commands())
	}

	fmt.Printf("secure conveyor simulator listening on %s\n", listener.Addr())
	fmt.Printf("public test-only Noise key: %s\n", simulator.DefaultTestEncryptionKey)
	if err := device.Serve(listener); err != nil {
		_ = device.Close()
		log.Fatal(err)
	}
}

func printCommands(commands <-chan proto.Message) {
	for command := range commands {
		printCommand(command)
	}
}

func printCommand(command proto.Message) {
	switch value := command.(type) {
	case *pb.FanCommandRequest:
		direction := "forward"
		if value.Direction == pb.FanDirection_FAN_DIRECTION_REVERSE {
			direction = "reverse"
		}
		fmt.Printf("received fan command: state=%t speed=%d direction=%s\n", value.State, value.SpeedLevel, direction)
	case *pb.LightCommandRequest:
		// The effect is the interesting half of this command for the conveyor,
		// so print it rather than only the color underneath it.
		fmt.Printf("received light command: state=%t brightness=%.2f rgb=#%02x%02x%02x effect=%q\n",
			value.State, value.Brightness, colorByte(value.Red), colorByte(value.Green), colorByte(value.Blue), value.Effect)
	}
}

func colorByte(value float32) uint8 {
	value = min(max(value, 0), 1)
	return uint8(math.Round(float64(value * 255)))
}
