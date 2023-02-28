package main

// Contents of the config file
type Config struct {
	// HomeAssistant connection params
	Hass HassConfig `yaml:"hass"`

	// One entity per phase, providing watts. Negative means power going into the grid.
	SmartmeterEntities []string `yaml:"smartmeterEntities"`

	// The system gives max priority to the battery.
	// If the battery isn't fully charged and isn't absorbing its full charging power,
	// then some of the controlled consumers may be turned off.
	Battery BatteryConfig `yaml:"battery"`

	// Devices to automatically control, with decreasing priority
	Consumers []Consumer `yaml:"consumers"`
}

type HassConfig struct {
	// E.g. "127.0.0.1"
	Address string `yaml:"address"`

	// Defaults to 8123
	Port int `yaml:"port"`

	// Authentication token
	Token string `yaml:"token"`

	// E.g. "zone.home"
	Zone string `yaml:"zone"`
}

type BatteryConfig struct {
	// Entity providing 0-100 charge pct
	PctEntity string `yaml:"pctEntity"`

	// Battery power in watts, negative meaning charging, positive meaning discharging
	PowerEntity string `yaml:"powerEntity"`

	// Max power in watts that the battery can absorb unless fully charged
	MaxChargingPower int `yaml:"maxChargingPower"`

	// Until what % of battery charge should the battery be expected to accept MaxChargingPower, e.g. 94
	FullChargePct int `yaml:"fullChargePct"`
}

type Consumer struct {
	// Entity to control
	// For type "binary", the entity should be a "switch.XXX" or "light.XXX"
	// For type "linear", the entity should be a "number.XXX", accepting values between 0.0 and 1.0
	Entity string `yaml:"entity"`

	// User friendly name
	Name string `yaml:"name"`

	// Maximum power consumption in watts
	Power int `yaml:"power"`

	// "binary" (default; device can be turned on and off)
	// "linear" (device power can be controlled between 0.0 and 1.0)
	Type string `yaml:"type"`

	// For linear devices:
	// Don't send less than this many watts to the device, turn it off instead
	MinPower int `yaml:"minPower"`

	// For binary devices:
	// When this device gets turned on, this is the minimum time it should remain on.
	// This is to prevent the device from being turned on/off rapidly.
	MinOnMinutes int `yaml:"minOnMinutes"`

	// For binary devices:
	// If this device has 2300W of power, we have a budget of 2000W and AllowBuyPower is 300W,
	// then it will be turned on.
	// It is reasonable to set this to a non-zero value, if you rather want to buy a little than sell a lot.
	AllowBuyPower int `yaml:"allowBuyPower"`

	// After toggling this device, wait this many seconds before taking further actions.
	// E.g. if this device is a whirlpool that takes 10 seconds to turn on or off and start/stop consuming power, set it to 10.
	// If this is a linear boiler, then this could be just 1 or 2 seconds.
	DelaySeconds int `yaml:"delaySeconds"`
}
