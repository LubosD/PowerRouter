package main

import (
	"log"
	"strings"
	"time"

	ga "saml.dev/gome-assistant"
)

// Simulates that power is being exported to the grid to opportunistically enable consumers.
// Useful in inverters settings where exports are disabled.
type ExportSimulator struct {
	ExportEnabledInverterModeEntity string
	exportDisabled                  bool
	blockedSince                    time.Time
	accumulatedValue                int

	lastStepUp *time.Time
}

const OPPORTUNISTIC_RETRY_INTERVAL = time.Minute * 2
const OPPORTUNISTIC_ZERO_POWER = 50
const OPPORTUNISTIC_STEP = 200 // watts

// Inverters can be incredibly slow when exporting power is forbidden
// Even slower when battery SoC is 99-100%.
const INVERTER_RAMP_UP_DELAY = time.Second * 15

func (es *ExportSimulator) Setup(gaApp *ga.App) {
	if es.ExportEnabledInverterModeEntity != "" {
		if es.ExportEnabledInverterModeEntity == "off" {
			// Permanently disabled
			es.exportDisabled = true
			log.Println("ExportSimulator: exports permanently disabled")
		} else {
			listener2 := ga.
				NewEntityListener().
				EntityIds(es.ExportEnabledInverterModeEntity).
				Call(func(s1 *ga.Service, s2 *ga.State, ed ga.EntityData) {
					es.exportDisabled = strings.ToLower(ed.ToState) == "off"
					log.Println("ExportSimulator: exports state now " + ed.ToState)
				}).
				RunOnStartup().
				Build()
			gaApp.RegisterEntityListeners(listener2)
		}
	}
}

func (es *ExportSimulator) Process(realMeasurement int) int {
	if es.lastStepUp != nil && time.Since(*es.lastStepUp) < INVERTER_RAMP_UP_DELAY && realMeasurement > 0 && es.accumulatedValue == 0 {
		// After faking an export and causing some device to turn on,
		// give the inverter time to ramp up power production
		log.Println("ExportSimulator: giving inverter time to ramp up production")
		return 0
	}

	if es.exportDisabled && realMeasurement > 0 {
		if realMeasurement < OPPORTUNISTIC_ZERO_POWER {
			if time.Since(es.blockedSince) > OPPORTUNISTIC_RETRY_INTERVAL {
				// Simulate an ongoing export
				simValue := es.accumulatedValue - OPPORTUNISTIC_STEP

				log.Printf("ExportSimulator: simulating power balance of %dW\n", simValue)

				now := time.Now()
				es.lastStepUp = &now

				return simValue
			}
		} else {
			// Power is being drawn from the grid, don't do anything and hold off simulating export
			es.blockedSince = time.Now()
			es.lastStepUp = nil
			log.Println("ExportSimulator: power drawn from grid -> inactive")
		}
	}
	return realMeasurement
}

func (es *ExportSimulator) UndistributedPower(watts int) {
	if es.exportDisabled {
		log.Printf("ExportSimulator: simulated %dW were not distributed", watts)
		es.accumulatedValue = watts
	}
}

func (es *ExportSimulator) DistributedPower(watts int) {
	if es.exportDisabled {
		now := time.Now()
		es.lastStepUp = &now
	}
}
