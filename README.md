# PV Power Router for Home Assistant

## Design goals

* Replace various HW power routers with a software based solution.
* Support both on/off devices as well as devices with variable power output.
* Respect device priorities and minimize exported power.

## Requirements

* You need to have up-to-date power consumption of the house, ideally directly from a smart meter.
  * If you have an Eastron smart meter already hooked up to the PV inverter, I suggest taking a look at my [eastron-modbus-wiretap](https://github.com/LubosD/eastron-modbus-wiretap) project, which will enable you to get the same data as the inverter is using.

* If you want the power router to also consider and prioritize battery charging, you need to have SoC (state of charge) and (dis)charge power data, either directly from the BMS (probably better) or from the inverter.

## Caveats

* By using this project, you're essentially adding a second power balancer to your PV system. As inverters have delays reacting to power consumption changes, they may end up doing a bit of back and forth with this power router until things stabilize. For example, this is why the config file has various configurable delays.

* There is **no UI** for now. All configuration must be done manually.

* **All entities** referenced in the config must be for the **sole use of this application**.
  * In other words, if you have a boiler with a variable power SSR controlled by esphome's climate controls, you will need to introduce a separate entity to push power into the boiler when excess power is available. If you do not do that, this power router will keep setting its power output to zero and your boiler will never heat unless the sun is shining. If you're building such a boiler, take a peek at [my config file](examples/ssr-boiler/dumbboiler.yaml) for that.
  * Also take a look at [my smart boiler example](examples/smart-boiler/).

## Configuration

This application must be configured by writing a YAML configuration file. See [examples](examples). Read the [config definition](config.go) for more information.

## Build

This application is written in [Go](https://go.dev/). To build it, you need to install Go and then run:
```
go build
```

The application takes an only argument, which is the path to the YAML config file.

## TODOs

* Power re-routing. Currently, the app will **not** re-route power from a lower priority device to a higher priority device.
* For countries with per-phase billing (e.g. Czech Republic), support considering inverter's maximum asymmetry.
