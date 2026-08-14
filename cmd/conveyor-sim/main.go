// Command conveyor-sim demonstrates the library without hardware or secrets.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	api "github.com/flavio-fernandes/go-aioesphomeapi"
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
)

func main() {
	device := simulator.New(simulator.ConveyorScenario())
	defer device.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := api.DialWithContext(ctx, "in-process", time.Second, device.ClientOptions()...)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	descriptors, err := client.ListEntities()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("connected securely to %s; discovered %d entities\n", client.Name(), len(descriptors))
	unsubscribe, err := client.SubscribeStates(nil)
	if err != nil {
		log.Fatal(err)
	}
	defer unsubscribe()
	if err := client.SetFan(simulator.ConveyorFanKey, api.FanCommandOpts{HasState: true, State: true, HasSpeedLevel: true, SpeedLevel: 100, HasDirection: true, Direction: pb.FanDirection_FAN_DIRECTION_FORWARD}); err != nil {
		log.Fatal(err)
	}
	// Selecting an effect by name is how the conveyor light works: the device
	// owns the animation and the controller only names the one it wants.
	if err := client.SetLight(simulator.StatusLightKey, api.LightCommandOpts{HasState: true, State: true, HasBrightness: true, Brightness: 0.35, HasColourMode: true, ColourMode: api.ColourModeRGB, HasRGB: true, Red: 1, Green: 1, Blue: 1, HasEffect: true, Effect: simulator.ConveyorEffectTraveling}); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("simulated conveyor belt on at speed=100 and status effect=%q\n", simulator.ConveyorEffectTraveling)
}
