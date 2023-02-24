package main

import (
	"errors"
	"log"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
	ga "saml.dev/gome-assistant"
)

var configuration Config

func main() {
	if len(os.Args) != 2 {
		println("Usage: PowerRouter /path/to/config.yaml")
		os.Exit(1)
	}

	err := loadConfig(os.Args[1])
	if err != nil {
		log.Fatalln("Error loading config:", err)
	}

	// HASS setup
	app := ga.NewApp(ga.NewAppRequest{
		IpAddress:        configuration.Hass.Address,
		Port:             strconv.Itoa(configuration.Hass.Port),
		HAAuthToken:      configuration.Hass.Token,
		HomeZoneEntityId: configuration.Hass.Zone,
	})
	defer app.Cleanup()

	// Power router setup
	router := &Router{
		Devices: make([]Device, len(configuration.Consumers)),
	}

	// Get ga.Service, currently not exported
	//service := reflect.ValueOf(app).Elem().FieldByName("service").Interface().(*ga.Service)

	// Instantiate devices to consume power
	for i, cons := range configuration.Consumers {
		switch cons.Type {
		case "binary":
			router.Devices[i] = &BinaryDevice{
				App:      app,
				Consumer: &cons,
			}
		case "linear":
			router.Devices[i] = &LinearDevice{
				App:      app,
				Consumer: &cons,
			}
		default:
			panic("Device " + cons.Name + " has unknown type: " + cons.Type)
		}

		router.Devices[i].Setup()
	}

	router.SmartMeter = &SmartMeter{
		Entities: configuration.SmartmeterEntities,
	}
	router.SmartMeter.Setup(app)

	router.Battery = &Battery{
		Config: &configuration.Battery,
	}
	router.Battery.Setup(app)

	// Initialize the router
	router.Setup()

	app.Start()
}

func loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, &configuration)
	if err != nil {
		return err
	}

	if len(configuration.SmartmeterEntities) < 1 {
		return errors.New("must define at least 1 smartmeter entity")
	}

	if configuration.Hass.Port == 0 {
		configuration.Hass.Port = 8123
	}

	return nil
}
