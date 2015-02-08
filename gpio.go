// +build !linux,!arm

package main

type GPIO struct {
	pinStates map[string] PinState
	pinStateChanged chan PinState
	pinAdded chan PinState
	pinRemoved chan string
}

func (g *GPIO) Init(pinStateChanged chan PinState, pinAdded chan PinState, pinRemoved chan string, states map[string] PinState) error {
	g.pinStateChanged = pinStateChanged
	g.pinRemoved = pinRemoved
	g.pinAdded = pinAdded
	g.pinStates = states

	// now init pins
	for key, pinState := range g.pinStates {
		if pinState.Name == "" {
			pinState.Name = pinState.PinId
		}
		g.PinInit(key, pinState.Dir, pinState.Pullup, pinState.Name)
		g.PinSet(key, pinState.State)
	}		
	return nil
}

func (g *GPIO) Close() error {
	return nil
}
func (g *GPIO) PinMap() ([]PinDef, error) {
	// return a mock pinmap for this mock interface
	pinmap := []PinDef {
		{
			"P8_07",
			[]string {"66", "GPIO_66", "TIMER4"},
			[]string {"analog","digital","pwm"},
			66,
			0,
		}, {
			"P8_08",
			[]string {"67", "GPIO_67", "TIMER7"},
			[]string {"analog","digital","pwm"},
			67,
			0,
		}, {
			"P8_09",
			[]string {"69", "GPIO_69", "TIMER5"},
			[]string {"analog","digital","pwm"},
			69,
			0,
		}, {
			"P8_10",
			[]string {"68", "GPIO_68", "TIMER6"},
			[]string {"analog","digital","pwm"},
			68,
			0,
		}, {
			"P8_11",
			[]string {"45", "GPIO_45"},
			[]string {"analog","digital","pwm"},
			45,
			0,
		},
	}
	return pinmap, nil
}
func (g *GPIO) Host() (string, error) {
	return "fake", nil
}
func (g *GPIO) PinStates() (map[string] PinState, error) {
	return g.pinStates, nil
}
func (g *GPIO) PinInit(pinId string, dir Direction, pullup PullUp, name string) error {
	// add a pin

	// look up internal ID (we're going to assume its correct already)

	// make a pinstate object
	pinState := PinState {
		nil,
		pinId,
		dir,
		0,
		pullup,
		name,
	}

	g.pinStates[pinId] = pinState

	g.pinAdded <- pinState
	return nil
}
func (g *GPIO) PinSet(pinId string, val byte) error {
	// change pin state
	if pin,ok := g.pinStates[pinId]; ok {
		// we have a value....
		pin.State = val
		g.pinStates[pinId] = pin
		// notify channel of new pinstate
		g.pinStateChanged <- pin
	}
	return nil
}
func (g *GPIO) PinRemove(pinId string) error {
	// remove a pin
	if _,ok := g.pinStates[pinId]; ok {
		// normally you would close the pin here
		delete(g.pinStates,pinId)
		g.pinRemoved <- pinId
	}
	return nil
}	