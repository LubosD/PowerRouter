package main

import (
	"errors"
	"log"
	"os"
	"strconv"
	"time"

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

	for {
		runApp()

		log.Println("Will restart in 5 seconds...")
		time.Sleep(5 * time.Second)
	}
}

func runApp() {
	// HASS setup
	app, err := ga.NewApp(ga.NewAppRequest{
		IpAddress:        configuration.Hass.Address,
		Port:             strconv.Itoa(configuration.Hass.Port),
		HAAuthToken:      configuration.Hass.Token,
		HomeZoneEntityId: configuration.Hass.Zone,
	})

	if err != nil {
		if errors.Is(err, ga.ErrInvalidToken) {
			log.Fatalln("Invalid HASS authentication token!")
		} else {
			log.Println("Error connecting to HASS:", err)
			return
		}
	}

	defer app.Cleanup()

	// Power router setup
	router := &Router{
		Devices: make([]Device, len(configuration.Consumers)),
	}

	// Instantiate devices to consume power
	for i := range configuration.Consumers {
		cons := &configuration.Consumers[i]

		switch cons.Type {
		case "binary":
			router.Devices[i] = &BinaryDevice{
				App:      app,
				Consumer: cons,
			}
		case "linear":
			router.Devices[i] = &LinearDevice{
				App:      app,
				Consumer: cons,
			}
		default:
			panic("Device " + cons.Name + " has unknown type: " + cons.Type)
		}

		router.Devices[i].Setup()
	}

	if configuration.ExportEnabledEntity != "" && configuration.ExportEnabledEntity != "on" {
		router.ExportSimulator = &ExportSimulator{
			ExportEnabledInverterModeEntity: configuration.ExportEnabledEntity,
		}
		router.ExportSimulator.Setup(app)
	} else {
		log.Println("Not configuring ExportSimulator")
	}

	router.SmartMeter = &SmartMeter{
		Entities: configuration.SmartmeterEntities,
	}
	router.SmartMeter.Setup(app)
	router.GlobalEnableEntity = configuration.GlobalEnableEntity

	if configuration.Battery != nil {
		router.Battery = &Battery{
			Config: configuration.Battery,
		}
		router.Battery.Setup(app)
	}

	// Initialize the router
	router.Setup(app)

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
