package main

import (
	"log"
	"strconv"
	"time"

	ga "saml.dev/gome-assistant"
)

type Battery struct {
	Config *BatteryConfig

	// SOC
	ChargePct int

	// Current discharge/charge power in watts
	CurrentPower int

	LastDataAt time.Time

	LoadFirst bool
}

func (b *Battery) Setup(gaApp *ga.App) {
	b.ChargePct = -1

	listenerPct := ga.
		NewEntityListener().
		EntityIds(b.Config.PctEntity).
		Call(b.handlePct).
		RunOnStartup().
		Build()

	listenerPower := ga.
		NewEntityListener().
		EntityIds(b.Config.PowerEntity).
		Call(b.handlePower).
		Build()

	if b.Config.LoadFirstEntity != "" {
		listenerLoadFirst := ga.
			NewEntityListener().
			EntityIds(b.Config.LoadFirstEntity).
			Call(b.handleLoadFirst).
			RunOnStartup().
			Build()
		gaApp.RegisterEntityListeners(listenerLoadFirst)
	}

	gaApp.RegisterEntityListeners(listenerPct, listenerPower)
}

func (b *Battery) handlePct(service *ga.Service, state ga.State, sensor ga.EntityData) {
	val, err := strconv.ParseFloat(sensor.ToState, 32)
	if err != nil {
		log.Printf("Cannot parse battery SOC value (%s): %v\n", sensor.ToState, err)
	} else {
		newVal := int(val)
		if newVal != b.ChargePct {
			b.ChargePct = int(val)
			log.Printf("Received new battery SoC: %d%%\n", b.ChargePct)
		}
	}
}

func (b *Battery) handlePower(service *ga.Service, state ga.State, sensor ga.EntityData) {
	val, err := strconv.ParseFloat(sensor.ToState, 32)
	if err != nil {
		log.Printf("Cannot parse battery power value (%s): %v\n", sensor.ToState, err)
	} else {
		b.CurrentPower = int(val)
		b.LastDataAt = sensor.LastChanged
	}
}

func (b *Battery) handleLoadFirst(service *ga.Service, state ga.State, sensor ga.EntityData) {
	if sensor.ToState == "off" {
		b.LoadFirst = false
	} else if sensor.ToState == "on" {
		b.LoadFirst = true
	}
}
