package main

import (
	"log"
	"strconv"

	ga "saml.dev/gome-assistant"
)

type Battery struct {
	Config *BatteryConfig

	// SOC
	ChargePct int

	// Current discharge/charge power in watts
	CurrentPower int
}

func (b *Battery) Setup(gaApp *ga.App) {
	listenerPct := ga.
		NewEntityListener().
		EntityIds(b.Config.PctEntity).
		Call(b.handlePct).
		Build()

	listenerPower := ga.
		NewEntityListener().
		EntityIds(b.Config.PowerEntity).
		Call(b.handlePower).
		Build()

	gaApp.RegisterEntityListeners(listenerPct, listenerPower)
}

func (b *Battery) handlePct(service *ga.Service, state *ga.State, sensor ga.EntityData) {
	val, err := strconv.ParseFloat(sensor.ToState, 32)
	if err != nil {
		log.Printf("Cannot parse battery SOC value (%s): %v\n", sensor.ToState, err)
	} else {
		b.ChargePct = int(val)
	}
}

func (b *Battery) handlePower(service *ga.Service, state *ga.State, sensor ga.EntityData) {
	val, err := strconv.ParseFloat(sensor.ToState, 32)
	if err != nil {
		log.Printf("Cannot parse battery power value (%s): %v\n", sensor.ToState, err)
	} else {
		b.CurrentPower = int(val)
	}
}
