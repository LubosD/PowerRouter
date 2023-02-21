package main

type Router struct {
	SmartMeter *SmartMeter
	Battery    *Battery
	Devices    map[string]*ConsumerDevice
}

func (r *Router) Setup() {
	r.SmartMeter.OnGridPower = r.rebalance
}

func (r *Router) rebalance(watts int) {

}
