package main

import (
	"log"
	"strings"

	ga "saml.dev/gome-assistant"
)

type LinearDevice struct {
	// Hass
	Service *ga.Service

	// Configuration data of this device
	Consumer *Consumer

	state float32
}

func (d *LinearDevice) Name() string {
	return d.Consumer.Name
}

func (d *LinearDevice) setPower(watts int) {
	d.state = float32(watts) / float32(d.Consumer.Power)

	if d.state > 1.0 {
		d.state = 1.0
	} else if d.state < 0 {
		d.state = 0.0
	}

	// Set this state to HASS
	entityType, _, _ := strings.Cut(d.Consumer.Entity, ".")

	switch entityType {
	case "number":
		d.Service.Number.SetValue(d.Consumer.Entity, d.state)
	default:
		log.Println("Don't know how to set power of entity type " + entityType)
	}
}

func (d *LinearDevice) getPower() int {
	return int(d.state * float32(d.Consumer.Power))
}

func (d *LinearDevice) TryConsumePower(watts int) bool {
	if watts <= int(1.0-d.state)*d.Consumer.Power {
		d.setPower(d.getPower() + watts)
		return true
	}
	return false
}

func (d *LinearDevice) TrySavePower(watts int) bool {
	if int(d.state*float32(d.Consumer.Power)) >= watts {
		d.setPower(d.getPower() - watts)
		return true
	}
	return false
}

func (d *LinearDevice) DelaySeconds() int {
	return d.Consumer.DelaySeconds
}
