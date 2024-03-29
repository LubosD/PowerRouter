package main

import (
	"log"
	"time"

	ga "saml.dev/gome-assistant"
)

// This many watts and less is considered to be zero
const BATTERY_ZERO_POWER = 10

type Router struct {
	SmartMeter         *SmartMeter
	Battery            *Battery
	Devices            []Device
	ExportSimulator    *ExportSimulator
	GlobalEnableEntity string

	noActionUntil              time.Time
	waitForNewBatteryDataAfter *time.Time
	disabled                   bool
}

func (r *Router) Setup(gaApp *ga.App) {
	r.SmartMeter.OnGridPower = r.rebalance

	if r.GlobalEnableEntity != "" {
		listener := ga.
			NewEntityListener().
			EntityIds(r.GlobalEnableEntity).
			Call(r.handleGlobalEnable).
			RunOnStartup().
			Build()

		gaApp.RegisterEntityListeners(listener)
	}
}

func (r *Router) handleGlobalEnable(service *ga.Service, state *ga.State, sensor ga.EntityData) {
	if sensor.ToState == "off" {
		r.disabled = true
		r.turnAllOff()
	} else if sensor.ToState == "on" {
		r.disabled = false
	}
}

func (r *Router) turnAllOff() {
	for _, device := range r.Devices {
		if power := device.CurrentPower(); power > 0 {
			device.TrySavePower(power)
		}
	}
}

func (r *Router) rebalance(watts int) {
	if r.disabled {
		return
	}
	if r.noActionUntil.After(time.Now()) {
		return
	}

	if r.waitForNewBatteryDataAfter != nil {
		if !r.Battery.LastDataAt.After(*r.waitForNewBatteryDataAfter) {
			log.Println("Awaiting new battery (dis)charge data before continuing")
			return
		}
		r.waitForNewBatteryDataAfter = nil
	}

	didBatteryAdj := false

	if r.Battery != nil {
		// If we have a battery, the battery isn't fully charged and isn't charging at full power,
		// then we should add the missing charge power to current grid power, because we prefer storing into battery
		// over "wasting" it on idle load.
		if r.Battery.ChargePct != -1 && r.Battery.ChargePct < r.Battery.Config.FullChargePct && -r.Battery.CurrentPower < r.Battery.Config.MaxChargingPower {
			// Adjust our import power with how many watts could theoretically go into the battery instead
			adj := r.Battery.Config.MaxChargingPower + r.Battery.CurrentPower

			// Adjust gradually to avoid going back and forth all the time
			adj /= 2

			watts += adj
			didBatteryAdj = true

			log.Printf("Battery charge is only %d%%, adjusting balance by %dW to %dW\n", r.Battery.ChargePct, adj, watts)
		} else if r.Battery.CurrentPower > BATTERY_ZERO_POWER && r.Battery.ChargePct < 99 {
			// Also, we should not use the battery charge to power our idle load.
			// Adjust our import power with how much the battery provides. This ensures we kill any optional devices.

			adj := r.Battery.CurrentPower

			// Adjust gradually to avoid going back and forth all the time
			adj /= 2

			watts += adj
			didBatteryAdj = true

			log.Printf("Battery is feeding into the load, adjusting balance by %dW to %dW\n", adj, watts)
		}
	}

	if r.ExportSimulator != nil {
		watts = r.ExportSimulator.Process(watts)
	}

	adjustedConsumption := false
	if watts < 0 {
		// We have excess power going into the grid, let's look for something to turn on

		budgetWatts := -watts
		for _, device := range r.Devices {
			if device.TryConsumePower(budgetWatts) {
				log.Printf("Increasing power consumption of [%s] with budget %d W\n", device.Name(), budgetWatts)

				r.noActionUntil = time.Now().Add(time.Second * time.Duration(device.DelaySeconds()))
				adjustedConsumption = true
				break
			}
		}

		if r.ExportSimulator != nil {
			if !adjustedConsumption {
				// Inform the simulator that this value didn't have any effect
				r.ExportSimulator.UndistributedPower(watts)
			} else {
				r.ExportSimulator.DistributedPower(watts)
			}
		}
	} else if watts > 0 {
		// We're buying power from the grid, let's see if we should maybe turn something off

		for i := len(r.Devices) - 1; i >= 0; i-- {
			device := r.Devices[i]
			if device.TrySavePower(watts) {
				log.Printf("Decreasing power consumption of [%s] due to excess of %d W\n", device.Name(), watts)

				r.noActionUntil = time.Now().Add(time.Second * time.Duration(device.DelaySeconds()))
				adjustedConsumption = true

				break
			}
		}
	}

	if didBatteryAdj && adjustedConsumption {
		// We adjusted house's power balance based on battery information and then we took action.
		// Because battery power info may be delayed (arriving at longer intervals) than smartmeter data,
		// we should avoid taking actions until new data comes so that we don't adjust with old data during next iteration.
		lastDataAt := time.Now()
		r.waitForNewBatteryDataAfter = &lastDataAt
	}
}
