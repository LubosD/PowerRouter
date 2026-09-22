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
	MaximizeUsage      bool

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

func (r *Router) handleGlobalEnable(service *ga.Service, state ga.State, sensor ga.EntityData) {
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
		if !r.Battery.LoadFirst && r.Battery.ChargePct != -1 && r.Battery.ChargePct < r.Battery.Config.FullChargePct && -r.Battery.CurrentPower < r.Battery.Config.MaxChargingPower {
			// Adjust our import power with how many watts could theoretically go into the battery instead
			adj := r.Battery.Config.MaxChargingPower + r.Battery.CurrentPower

			// Adjust gradually to avoid going back and forth all the time
			adj /= 2

			watts += adj
			didBatteryAdj = true

			log.Printf("Battery charge is only %d%%, adjusting balance by %dW to %dW\n", r.Battery.ChargePct, adj, watts)
		} else if r.Battery.LoadFirst && -r.Battery.CurrentPower > BATTERY_ZERO_POWER {
			log.Printf("Load first and the battery is charging at %dW - increasing load", -r.Battery.CurrentPower)

			adj := r.Battery.CurrentPower / 2

			watts += adj
			didBatteryAdj = true
		} else if r.Battery.CurrentPower > BATTERY_ZERO_POWER && r.Battery.ChargePct < 100 {
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

		if r.MaximizeUsage {
			budgetWatts := -watts
			if r.Battery != nil && r.Battery.MinBatteryPower > 0 &&
				r.Battery.ChargePct != -1 && r.Battery.ChargePct < r.Battery.Config.FullChargePct {
				budgetWatts -= r.Battery.MinBatteryPower
				log.Printf("Reserving %dW for battery (SoC %d%%), MaximizeUsage budget reduced to %dW\n",
					r.Battery.MinBatteryPower, r.Battery.ChargePct, budgetWatts)
			}
			if budgetWatts > 0 {
				adjusted, delaySecs := r.rebalanceMaximize(budgetWatts)
				if adjusted {
					r.noActionUntil = time.Now().Add(time.Second * time.Duration(delaySecs))
					adjustedConsumption = true
				}
			}
		}

		if !adjustedConsumption {
			budgetWatts := -watts
			for _, device := range r.Devices {
				if device.TryConsumePower(budgetWatts) {
					log.Printf("Increasing power consumption of [%s] with budget %d W\n", device.Name(), budgetWatts)

					r.noActionUntil = time.Now().Add(time.Second * time.Duration(device.DelaySeconds()))
					adjustedConsumption = true
					break
				}
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

// rebalanceMaximize finds the allocation of power across all devices that maximizes total consumption.
// It enumerates all on/off combinations of flexible binary devices and greedily allocates remaining
// budget to linear devices in priority order.
// Returns whether any changes were made and the delay to apply.
func (r *Router) rebalanceMaximize(budgetWatts int) (adjusted bool, delaySecs int) {
	type flexBinary struct {
		device *BinaryDevice
		index  int
	}
	type flexLinear struct {
		device *LinearDevice
		index  int
	}

	var fixedCost int
	var flexBinaries []flexBinary
	var flexLinears []flexLinear

	// Classify devices and compute total redistributable budget
	totalBudget := budgetWatts
	for i, dev := range r.Devices {
		switch d := dev.(type) {
		case *BinaryDevice:
			if d.state && !d.turnOffAllowed() {
				// Fixed: must stay on, subtract from budget
				fixedCost += d.Consumer.Power
			} else {
				// Flexible: add current power back to budget
				totalBudget += d.CurrentPower()
				flexBinaries = append(flexBinaries, flexBinary{device: d, index: i})
			}
		case *LinearDevice:
			// Add current power back to budget
			totalBudget += d.CurrentPower()
			flexLinears = append(flexLinears, flexLinear{device: d, index: i})
		}
	}

	totalBudget -= fixedCost
	if totalBudget <= 0 {
		return false, 0
	}

	// Current total consumption across flexible devices
	currentTotal := 0
	for _, fb := range flexBinaries {
		currentTotal += fb.device.CurrentPower()
	}
	for _, fl := range flexLinears {
		currentTotal += fl.device.CurrentPower()
	}

	bestTotal := currentTotal
	bestBinaryMask := -1
	bestLinearAllocs := []int(nil)

	numBinaries := len(flexBinaries)
	numCombinations := 1 << numBinaries

	for mask := 0; mask < numCombinations; mask++ {
		// Calculate cost of binary devices in this combination
		binaryCost := 0
		for bit := 0; bit < numBinaries; bit++ {
			if mask&(1<<bit) != 0 {
				fb := flexBinaries[bit]
				effectiveCost := fb.device.Consumer.Power - fb.device.Consumer.AllowBuyPower
				binaryCost += effectiveCost
			}
		}

		if binaryCost > totalBudget {
			continue
		}

		// Remaining budget for linear devices
		remaining := totalBudget - binaryCost

		// Actual total power used by binary devices (not effective cost, real power)
		totalUsed := 0
		for bit := 0; bit < numBinaries; bit++ {
			if mask&(1<<bit) != 0 {
				totalUsed += flexBinaries[bit].device.Consumer.Power
			}
		}

		// Greedily allocate remaining budget to linear devices in priority order
		linearAllocs := make([]int, len(flexLinears))
		for li, fl := range flexLinears {
			if remaining <= 0 {
				break
			}
			maxPower := fl.device.Consumer.Power
			alloc := remaining
			if alloc > maxPower {
				alloc = maxPower
			}
			if alloc < fl.device.Consumer.MinPower {
				alloc = 0
			}
			linearAllocs[li] = alloc
			totalUsed += alloc
			remaining -= alloc
		}

		if totalUsed > bestTotal {
			bestTotal = totalUsed
			bestBinaryMask = mask
			bestLinearAllocs = linearAllocs
		}
	}

	if bestBinaryMask < 0 || bestTotal <= currentTotal {
		return false, 0
	}

	// Apply the best allocation
	maxDelay := 0
	changed := false

	for bit := 0; bit < numBinaries; bit++ {
		fb := flexBinaries[bit]
		wantOn := bestBinaryMask&(1<<bit) != 0
		if fb.device.state != wantOn {
			log.Printf("MaximizeUsage: setting [%s] to %v\n", fb.device.Name(), wantOn)
			fb.device.setPower(wantOn)
			changed = true
			if fb.device.Consumer.DelaySeconds > maxDelay {
				maxDelay = fb.device.Consumer.DelaySeconds
			}
		}
	}

	for li, fl := range flexLinears {
		newPower := bestLinearAllocs[li]
		if newPower != fl.device.CurrentPower() {
			log.Printf("MaximizeUsage: setting [%s] to %d W\n", fl.device.Name(), newPower)
			fl.device.setPower(newPower)
			changed = true
			if fl.device.Consumer.DelaySeconds > maxDelay {
				maxDelay = fl.device.Consumer.DelaySeconds
			}
		}
	}

	if changed {
		log.Printf("MaximizeUsage: total consumption %d W -> %d W (budget %d W)\n", currentTotal, bestTotal, budgetWatts)
	}

	return changed, maxDelay
}
