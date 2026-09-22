package main

import (
	"log"
	"math"
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
		if r.MaximizeUsage && r.Battery.MinBatteryPower > 0 && r.Battery.ChargePct != -1 && r.Battery.ChargePct < r.Battery.Config.FullChargePct {
			// In MaximizeUsage mode with a battery reservation, route all solar to loads
			// but guarantee at least MinBatteryPower goes to battery.
			// adj > 0 when battery is below reservation (shed load), adj < 0 when above (add load).
			adj := (r.Battery.CurrentPower + r.Battery.MinBatteryPower) / 2
			watts += adj
			didBatteryAdj = true
			log.Printf("MaximizeUsage: reserving %dW for battery (SoC %d%%, charging at %dW), adjusting balance by %dW to %dW\n",
				r.Battery.MinBatteryPower, r.Battery.ChargePct, -r.Battery.CurrentPower, adj, watts)
		} else if !r.Battery.LoadFirst && r.Battery.ChargePct != -1 && r.Battery.ChargePct < r.Battery.Config.FullChargePct && -r.Battery.CurrentPower < r.Battery.Config.MaxChargingPower {
			// If we have a battery, the battery isn't fully charged and isn't charging at full power,
			// then we should add the missing charge power to current grid power, because we prefer storing into battery
			// over "wasting" it on idle load.
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
			adjusted, delaySecs := r.rebalanceMaximize(-watts)
			if adjusted {
				r.noActionUntil = time.Now().Add(time.Second * time.Duration(delaySecs))
				adjustedConsumption = true
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

// binaryWeight returns a flexible binary device's "effective cost" against the power budget:
// the AllowBuyPower portion is considered free (willing to buy that much from the grid),
// so it doesn't count against the budget. Floored at 0 in case of a misconfigured device
// where AllowBuyPower > Power.
func binaryWeight(d *BinaryDevice) int {
	weight := d.Consumer.Power - d.Consumer.AllowBuyPower
	if weight < 0 {
		weight = 0
	}
	return weight
}

// rebalanceMaximize finds the allocation of power across all devices that maximizes total consumption.
// It solves a 0/1 knapsack over flexible binary devices to find, for every exact amount of budget that
// could be spent on them, the combination that buys the most "free" power (AllowBuyPower), then combines
// that with what the linear devices would greedily absorb from whatever budget is left over - checking
// every possible split between binaries and linears, since the linear devices' payoff is nonlinear (it
// saturates once their combined capacity is reached), so simply maximizing binary power alone is not
// always optimal.
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

	numBinaries := len(flexBinaries)

	// linearGreedyTotal returns the total power the flexible linear devices would
	// absorb, greedily in priority order, given `remaining` budget to spend.
	linearGreedyTotal := func(remaining int) int {
		total := 0
		for _, fl := range flexLinears {
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
			total += alloc
			remaining -= alloc
		}
		return total
	}

	// 0/1 knapsack: dp[w] = max total AllowBuyPower ("bonus") achievable from a subset
	// of flexBinaries whose effective cost (weight) sums to EXACTLY w (unreachable
	// sums are left as -inf). We need the exact spend - not just "at most" - because
	// what's left over falls through to the linear devices below.
	const unreachable = math.MinInt32 / 2
	dp := make([]int, totalBudget+1)
	for w := range dp {
		dp[w] = unreachable
	}
	dp[0] = 0
	included := make([][]bool, numBinaries)

	for i, fb := range flexBinaries {
		weight := binaryWeight(fb.device)
		bonus := fb.device.Consumer.Power - weight // AllowBuyPower, consistent with weight's clamping

		included[i] = make([]bool, totalBudget+1)
		for w := totalBudget; w >= weight; w-- {
			if dp[w-weight] > unreachable && dp[w-weight]+bonus > dp[w] {
				dp[w] = dp[w-weight] + bonus
				included[i][w] = true
			}
		}
	}

	// For every exact binary spend w, real binary power used is w+dp[w] (weight +
	// bonus = Power of the devices chosen); combine with what that leaves for the
	// linear devices and pick the split that maximizes the total.
	bestW := 0
	bestUsed := unreachable
	for w := 0; w <= totalBudget; w++ {
		if dp[w] <= unreachable {
			continue
		}
		used := w + dp[w] + linearGreedyTotal(totalBudget-w)
		if used > bestUsed {
			bestUsed = used
			bestW = w
		}
	}

	// Backtrack to recover which devices were selected for spend bestW.
	bestBinaryMask := 0
	w := bestW
	for i := numBinaries - 1; i >= 0; i-- {
		if included[i][w] {
			bestBinaryMask |= 1 << i
			w -= binaryWeight(flexBinaries[i].device)
		}
	}

	// Greedily allocate the remaining budget to linear devices in priority order.
	remaining := totalBudget - bestW
	totalUsed := bestUsed
	bestLinearAllocs := make([]int, len(flexLinears))
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
		bestLinearAllocs[li] = alloc
		remaining -= alloc
	}

	if totalUsed <= currentTotal {
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
		log.Printf("MaximizeUsage: total consumption %d W -> %d W (budget %d W)\n", currentTotal, totalUsed, budgetWatts)
	}

	return changed, maxDelay
}
