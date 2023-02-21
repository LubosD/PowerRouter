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
}

type Consumer struct {
	// Entity to control
	Entity string `yaml:"entity"`

	// User friendly name
	Name string `yaml:"name"`

	// Maximum power consumption in watts
	Power int `yaml:"power"`

	// "binary" (default; device can be turned on and off)
	// "linear" (device power can be controlled between 0.0 and 1.0)
	Type string `yaml:"type"`
}
