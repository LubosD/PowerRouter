package main

import (
	"log"
	"strings"
	"time"

	ga "saml.dev/gome-assistant"
)

type BinaryDevice struct {
	// Hass
	Service *ga.Service

	// Configuration data of this device
	Consumer *Consumer

	state bool

	lastTurnedOn time.Time
}

func (d *BinaryDevice) Name() string {
	return d.Consumer.Name
}

func (d *BinaryDevice) turnOffAllowed() bool {
	if d.Consumer.MinimumOnMinutes > 0 && time.Since(d.lastTurnedOn).Minutes() < float64(d.Consumer.MinimumOnMinutes) {
		return false
	}
	return true
}

func (d *BinaryDevice) setPower(on bool) {
	d.state = on

	// Set this state to HASS
	entityType, _, _ := strings.Cut(d.Consumer.Entity, ".")

	switch entityType {
	case "switch":
		if d.state {
			d.Service.Switch.TurnOn(d.Consumer.Entity)
		} else {
			d.Service.Switch.TurnOff(d.Consumer.Entity)
		}
	case "light":
		if d.state {
			d.Service.Light.TurnOn(d.Consumer.Entity)
		} else {
			d.Service.Light.TurnOff(d.Consumer.Entity)
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
