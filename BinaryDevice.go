package main

import (
	"log"
	"strconv"
	"strings"
	"time"

	ga "saml.dev/gome-assistant"
)

type BinaryDevice struct {
	// Hass
	App *ga.App

	// Configuration data of this device
	Consumer *Consumer

	state bool

	lastTurnedOn time.Time
}

func (d *BinaryDevice) Setup() {
	listener := ga.
		NewEntityListener().
		EntityIds(d.Consumer.Entity).
		Call(d.handleValue).
		Build()

	d.App.RegisterEntityListeners(listener)
}

func (d *BinaryDevice) handleValue(service *ga.Service, state *ga.State, sensor ga.EntityData) {
	switch sensor.ToState {
	case "0":
		d.state = false
	case "1":
		d.state = true
	default:
		value, err := strconv.ParseBool(sensor.ToState)

		if err == nil {
			d.state = value
		} else {
			log.Printf("Cannot parse current value of entity [%s]: %s\n", sensor.TriggerEntityId, sensor.ToState)
		}
	}
}

func (d *BinaryDevice) Name() string {
	return d.Consumer.Name
}

func (d *BinaryDevice) turnOffAllowed() bool {
	if d.Consumer.MinOnMinutes > 0 && time.Since(d.lastTurnedOn).Minutes() < float64(d.Consumer.MinOnMinutes) {
		return false
	}
	return true
}

func (d *BinaryDevice) setPower(on bool) {
	d.state = on

	// Set this state to HASS
	entityType, _, _ := strings.Cut(d.Consumer.Entity, ".")
	service := d.App.GetService()

	switch entityType {
	case "switch":
		if d.state {
			service.Switch.TurnOn(d.Consumer.Entity)
		} else {
			service.Switch.TurnOff(d.Consumer.Entity)
		}
	case "light":
		if d.state {
			service.Light.TurnOn(d.Consumer.Entity)
		} else {
			service.Light.TurnOff(d.Consumer.Entity)
		}
	default:
		log.Println("Don't know how to turn on/off entity type " + entityType)
	}
}

func (d *BinaryDevice) TryConsumePower(watts int) bool {
	if !d.state {
		if watts >= (d.Consumer.Power - d.Consumer.AllowBuyPower) {
			d.setPower(true)
			return true
		}
	}
	return false
}

func (d *BinaryDevice) TrySavePower(watts int) bool {
	if d.state {
		if !d.turnOffAllowed() {
			return false
		}
		if watts > d.Consumer.AllowBuyPower {
			d.setPower(false)
			return true
		}
	}
	return false
}

func (d *BinaryDevice) DelaySeconds() int {
	return d.Consumer.DelaySeconds
}
