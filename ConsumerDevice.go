package main

import (
	"fmt"

	ga "saml.dev/gome-assistant"
)

type ConsumerDevice struct {
	// Hass app instance
	App *ga.App

	// Configuration data of this device
	Consumer *Consumer

	stateLinear float32
	stateBinary bool
}

func (d *ConsumerDevice) SetStateLinear(value float32) {
	if value < 0 || value > 1 {
		panic(fmt.Sprint("Invalid linear value passed: ", value))
	}

	if d.Consumer.Type != "linear" {
		panic("SetStateLinear() called on non-linear device")
	}

	// TODO: set value

	d.stateLinear = value
}

func (d *ConsumerDevice) SetStateBinary(value bool) {

	if d.Consumer.Type != "binary" {
		panic("SetStateBinary() called on non-binary device")
	}

	// TODO: set value

	d.stateBinary = value
}

func (d *ConsumerDevice) GetStateLinear() float32 {
	return d.stateLinear
}

func (d *ConsumerDevice) GetStateBinary() bool {
	return d.stateBinary
}
