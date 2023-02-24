package main

import (
	"log"
	"time"
)

type Router struct {
	SmartMeter *SmartMeter
	Battery    *Battery
	Devices    []Device

	noActionUntil time.Time
}

func (r *Router) Setup() {
	r.SmartMeter.OnGridPower = r.rebalance
}

func (r *Router) rebalance(watts int) {
	if r.noActionUntil.After(time.Now()) {
		return
	}

	if watts < 0 {
		// We have excess power going into the grid, let's look for something to turn on

		budgetWatts := -watts
		for _, device := range r.Devices {
			if device.TryConsumePower(budgetWatts) {
				log.Printf("Increasing power consumption of [%s] with budget %d W\n", device.Name(), budgetWatts)
				r.noActionUntil = time.Now().Add(time.Second * time.Duration(device.DelaySeconds()))
				break
			}
		}
	} else {
		// We're buying power from the grid, let's see if we should maybe turn something off

		if r.Battery != nil {
			// If we have a battery, the battery isn't fully charged and isn't charging at full power,
			// then we should add the missing charge power to current grid power, because we prefer storing into battery
			// over "wasting" it on idle load.
			// Also, we should not use the battery charge to power our idle load.
			if r.Battery.ChargePct < 99 && -r.Battery.CurrentPower < r.Battery.Config.MaxChargingPower {
				watts += r.Battery.Config.MaxChargingPower + r.Battery.CurrentPower
			}
		}

		for i := len(r.Devices) - 1; i >= 0; i-- {
			device := r.Devices[i]
			if device.TrySavePower(watts) {
				log.Printf("Decreasing power consumption of [%s] due to excess of %d W\n", device.Name(), watts)

				r.noActionUntil = time.Now().Add(time.Second * time.Duration(device.DelaySeconds()))
				break
			}
		}
	}
}
