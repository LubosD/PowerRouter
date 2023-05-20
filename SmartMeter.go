package main

import (
	"log"
	"strconv"
	"time"

	"golang.org/x/exp/slices"
	ga "saml.dev/gome-assistant"
)

const maxDuration time.Duration = 1<<63 - 1

type SmartMeter struct {
	Entities []string

	OnGridPower func(watts int)

	lastValues     []*float32
	reportingTimer *time.Timer
}

func (sm *SmartMeter) Setup(gaApp *ga.App) {
	sm.reportingTimer = time.AfterFunc(maxDuration, sm.reportValues)
	listener := ga.
		NewEntityListener().
		EntityIds(sm.Entities...).
		Call(sm.handleValues).
		RunOnStartup().
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

	sm.reportingTimer.Reset(250 * time.Millisecond)
}

func (sm *SmartMeter) reportValues() {
	var powerBalance float32
	for _, v := range sm.lastValues {
		if v == nil {
			// We don't have data from all sensors yet
			log.Println("SmartMeter: Don't have values for all phases yet")
			return
		}
		powerBalance += *v
	}

	log.Printf("Current house power balance: %d W\n", int(powerBalance))

	if sm.OnGridPower != nil {
		sm.OnGridPower(int(powerBalance))
	}
}
