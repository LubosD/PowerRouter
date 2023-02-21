package main

type Device interface {
	// Device name
	Name() string

	// Given this many watts, can you take care of them?
	// For LinearDevices, it considers how much power is currently sent to this device.
	// For BinaryDevices, it looks at the declared power minus "AllowBuyPower", if turned off.
	TryConsumePower(watts int) bool

	// Given this many watts we're currently buying, can you reduce your consumption by this much?
	// For LinearDevices, it considers how much power is currently sent to this device.
	// For BinaryDevices, it returns true if the device is turned on and watts is more than "AllowBuyPower".
	TrySavePower(watts int) bool

	// After successfully calling TryConsumePower/TrySavePower on this device, wait this many seconds before taking further actions.
	DelaySeconds() int
}
