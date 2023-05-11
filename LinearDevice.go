package main

import (
	"log"
	"strconv"
	"strings"

	ga "saml.dev/gome-assistant"
)

type LinearDevice struct {
	// Hass
	App *ga.App

	// Configuration data of this device
	Consumer *Consumer

	state float32
}

func (d *LinearDevice) Setup() {
	listener := ga.
		NewEntityListener().
		EntityIds(d.Consumer.Entity).
		Call(d.handleValue).
		RunOnStartup().
		Build()

	d.App.RegisterEntityListeners(listener)
}

func (d *LinearDevice) handleValue(service *ga.Service, state *ga.State, sensor ga.EntityData) {
	value, err := strconv.ParseFloat(sensor.ToState, 32)
	if err == nil {
		log.Printf("Received new value of entity [%s]: %s", sensor.TriggerEntityId, sensor.ToState)
		d.state = float32(value)
	} else {
		log.Printf("Cannot parse current value of entity [%s]: %s\n", sensor.TriggerEntityId, sensor.ToState)
	}
}

func (d *LinearDevice) Name() string {
	return d.Consumer.Name
}

func (d *LinearDevice) setPower(watts int) {
	if watts < d.Consumer.MinPower {
		watts = 0
	}
	d.state = float32(watts) / float32(d.Consumer.Power)

	if d.state > 1.0 {
		d.state = 1.0
	} else if d.state < 0 {
		d.state = 0.0
	}

	// Set this state to HASS
	entityType, _, _ := strings.Cut(d.Consumer.Entity, ".")
	service := d.App.GetService()

	switch entityType {
	case "number":
		service.Number.SetValue(d.Consumer.Entity, d.state)
	default:
		log.Println("Don't know how to set power of entity type " + entityType)
	}
}

func (d *LinearDevice) CurrentPower() int {
	return int(d.state * float32(d.Consumer.Power))
}

func (d *LinearDevice) TryConsumePower(watts int) bool {
	if d.state < 1.0 && d.CurrentPower()+watts > d.Consumer.MinPower {
		d.setPower(d.CurrentPower() + watts)
		return true
	}
	return false
}

func (d *LinearDevice) TrySavePower(watts int) bool {
	if d.state > 0 {
		d.setPower(d.CurrentPower() - watts)
		return true
	} else {
		// log.Printf("Cannot save %d watts, I can only do %d W\n", watts, int(d.state*float32(d.Consumer.Power)))
		return false
	}
}

func (d *LinearDevice) DelaySeconds() int {
	return d.Consumer.DelaySeconds
}
