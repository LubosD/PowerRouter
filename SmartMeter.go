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
	timerPending   bool
}

func (sm *SmartMeter) Setup(gaApp *ga.App) {
	listener := ga.
		NewEntityListener().
		EntityIds(sm.Entities...).
		Call(sm.handleValues).
		RunOnStartup().
		Build()

	gaApp.RegisterEntityListeners(listener)
}

func (sm *SmartMeter) handleValues(service *ga.Service, state ga.State, sensor ga.EntityData) {
	// log.Println("Received new smartmeter data:", sensor.TriggerEntityId, "=", sensor.ToState)
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

	if sm.lastValues == nil {
		sm.lastValues = make([]*float32, len(sm.Entities))
	}
	sm.lastValues[phaseIndex] = &f32

	if !sm.timerPending {
		sm.reportingTimer = time.AfterFunc(250*time.Millisecond, sm.onReportTimer)
		sm.timerPending = true
	}
}

func (sm *SmartMeter) onReportTimer() {
	sm.timerPending = false
	sm.reportValues()
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
