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
	if err := client.SetFan(simulator.ConveyorFanKey, api.FanCommandOpts{HasState: true, State: true, HasSpeedLevel: true, SpeedLevel: 35}); err != nil {
		log.Fatal(err)
	}
	if err := client.SetLight(simulator.StatusLightKey, api.LightCommandOpts{HasState: true, State: true, HasColorMode: true, ColorMode: pb.ColorMode_COLOR_MODE_RGB, HasRGB: true, Green: 1}); err != nil {
		log.Fatal(err)
	}
	fmt.Println("simulated conveyor speed=35 and status color=#00ff00")
}
