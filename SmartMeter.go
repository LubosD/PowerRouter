package main

import (
	"log"
	"strconv"

	"golang.org/x/exp/slices"
	ga "saml.dev/gome-assistant"
)

type SmartMeter struct {
	Entities []string

	OnGridPower func(watts int)

	lastValues []*float32
}

func (sm *SmartMeter) Setup(gaApp *ga.App) {
	listener := ga.
		NewEntityListener().
		EntityIds(sm.Entities...).
		Call(sm.handleValues).
		Build()

	gaApp.RegisterEntityListeners(listener)
}

func (sm *SmartMeter) handleValues(service *ga.Service, state *ga.State, sensor ga.EntityData) {
	if sm.lastValues == nil {
		sm.lastValues = make([]*float32, len(sm.Entities))
	}

	phaseIndex := slices.Index(sm.Entities, sensor.TriggerEntityId)
	if phaseIndex == -1 {
		panic("Received SM value change for unknown entity: " + sensor.TriggerEntityId)
	}
	value, err := strconv.ParseFloat(sensor.ToState, 32)

	if err != nil {
		log.Printf("Error parsing smartmeter data (%s) as float: %v\n", sensor.ToState, err)
		return
	}

	f32 := float32(value)
	sm.lastValues[phaseIndex] = &f32

	var powerBalance float32
	for _, v := range sm.lastValues {
		if v == nil {
			// We don't have data from all sensors yet
			return
		}
		powerBalance += *v
	}

	log.Printf("Current house power balance: %d W\n", int(powerBalance))

	if sm.OnGridPower != nil {
		sm.OnGridPower(int(powerBalance))
	}

	// Reset all data, wait for new events to fill it again
	for i := range sm.lastValues {
		sm.lastValues[i] = nil
	}
}
