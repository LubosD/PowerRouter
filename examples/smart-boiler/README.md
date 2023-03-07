This is the configuration you need to create to hook up the [Dražice smart boiler](https://github.com/LubosD/esphome-smartboiler) into this power router.

* Create a [template switch](switch.yaml) used to trigger automations. This shall go into Home Assistant's `configuration.yaml`
* Add these [automations](automations.yaml), adapting it to your device IDs etc.
* Hook the switch into the power router app.
