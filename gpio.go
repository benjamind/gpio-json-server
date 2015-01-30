
package main

import (
	"log"
	"encoding/json"
	"github.com/kidoman/embd"
	_ "github.com/kidoman/embd/host/all"
	"os"
	"io/ioutil"
)

type Direction int
type PullUp int

type PinState struct {
	Pin embd.DigitalPin `json:"-"`
	PinId string
	Dir Direction
	State int
	Pullup PullUp
	Name string
}
var pinStates = make(map[string] PinState)

const STATE_FILE = "pinstates.json"

const (
	In Direction = 0
	Out Direction = 1
	PWM Direction = 2

	Pull_None PullUp = 0
	Pull_Up PullUp = 1
	Pull_Down PullUp = 2
)

func gpioInit() {
	err := embd.InitGPIO()
	if err != nil {
		log.Println("Error in InitGPIO : " + err.Error())
	}
	// attempt to read pinstate file if found
	if _, err := os.Stat(STATE_FILE); err == nil {
		log.Println("Reading prexisting pinstate file : " + STATE_FILE)
		dat, err := ioutil.ReadFile(STATE_FILE)
		if err != nil {
			log.Println("Failed to read state file : " + STATE_FILE + " : " + err.Error())
			return
		}
		err = json.Unmarshal(dat, &pinStates)
		if err != nil {
			log.Println("Failed to unmarshal json : " + err.Error())
			return
		}
		// now initialise the pins as above
		for key, pinState := range pinStates {
			if pinState.Name == "" {
				pinState.Name = pinState.PinId
			}
			gpioInitPin(key, pinState.Dir, pinState.Pullup, pinState.Name)
			gpioSetPin(key, pinState.State)
		}		
	}
}
func gpioClose() {
	// close all the pins we have open if any
	for key, pinState := range pinStates {
		log.Println("Closing pin " + key)
		if pinState.Pin != nil {
			pinState.Pin.Close()
		}
	}

	err := embd.CloseGPIO()
	if err != nil {
		log.Println("Error in InitGPIO : " + err.Error())
	}

	data, err := json.Marshal(pinStates)
	if err!=nil {
		log.Println("Error marshalling pin states : " + err.Error())
		return
	}
	ioutil.WriteFile(STATE_FILE, data, 0644)
}
func gpioHost() {
	host, _, err := embd.DetectHost()
	if err != nil {
		log.Println(err)
		h.broadcastSys <- []byte("Error detecting gpio host " + err.Error())
		return
	}

	log.Println("Host = " + host)

	r, err := json.MarshalIndent(host, "", "\t")
	if err != nil {
		log.Println(err)
		h.broadcastSys <- []byte("Error marshalling gpio host " + err.Error())
	} else {
		log.Println("Sending " + string(r))
		h.broadcastSys <- r
	}
}
func gpioPinStates() {
	// wrap pinmap in a struct to make the json easier to parse on the other end
	pinstates := struct {
		PinStates map[string] PinState
		Success bool
		Type string
	}{
		pinStates,
		true,
		"PinStates",
	}
	r, err := json.MarshalIndent(pinstates, "", "\t")
	if err != nil {
		log.Println(err)
		h.broadcastSys <- []byte("Error marshalling gpio pinstates " + err.Error())
	} else {
		h.broadcastSys <- r
	}
}
func gpioPinMap() {
	desc, err := embd.DescribeHost()
	if err != nil {
		log.Println(err)
		h.broadcastSys <- []byte("Error describing gpio pinmap " + err.Error())
		return
	}
	
	// wrap pinmap in a struct to make the json easier to parse on the other end
	pinmap := struct {
		Pinmap embd.PinMap
		Success bool
		Type string
	}{
		desc.GPIODriver().GetPinMap(),
		true,
		"Pinmap",
	}

	r, err := json.MarshalIndent(pinmap, "", "\t")
	if err != nil {
		log.Println(err)
		h.broadcastSys <- []byte("Error marshalling gpio pinmap " + err.Error())
	} else {
		h.broadcastSys <- r
	}
}
func gpioPWMPin(pinId string, value byte) {
	// detect host to determine if we should use go-pi-blaster or embd
	host, _, err := embd.DetectHost()
	if err != nil {
		log.Println(err)
		h.broadcastSys <- []byte("Error detecting gpio host " + err.Error())
		return
	}
	if host == embd.HostRPi {
		// get pin map to lookup logical pin from string name
		desc, err := embd.DescribeHost()
		if err != nil {
			log.Println(err)
			h.broadcastSys <- []byte("Error describing gpio pinmap " + err.Error())
			return
		}
		
		pinmap := desc.GPIODriver().GetPinMap()

		pd, found := pinmap.Lookup(pinId, 1)

		if !found {
			log.Println("Pin " + pinId + " not found : ",(found))
			return
		}
		
		b.Apply(int64(pd.DigitalLogical), float64(value)/100.0)

	} else if host == embd.HostBBB {
		// UNTESTED!
		pwm, err := embd.NewPWMPin(pinId)
		if err != nil {
			log.Println("Failed to create PWM Pin on " + pinId + " : " + err.Error())
			h.broadcastSys <- []byte("Error creating PWM Pin " + err.Error())
			return
		}

		if err := pwm.SetAnalog(value); err != nil {
			log.Println("Failed to create PWM SetAnalog on " + pinId + " : " + err.Error())
			h.broadcastSys <- []byte("Error creating PWM SetAnalog " + err.Error())
		}
	}
}


func gpioInitPin(pinId string, dir Direction, pullup PullUp, name string) {
	pin, err := embd.NewDigitalPin(pinId)
	if err != nil {
		log.Println("Failed to create Digital Pin on " + pinId + " : " + err.Error())
		h.broadcastSys <- []byte("Error creating Digital Pin " + err.Error())
		return
	}
	err = pin.SetDirection(embd.Direction(dir))
	if err != nil {
		log.Println("Failed to create set direction on " + pinId + " : " + err.Error())
		h.broadcastSys <- []byte("Error setting direction " + err.Error())
		return	
	}

	state := 0

	if pullup == Pull_Up {
		err = pin.PullUp()	

		// pullup and down not implemented on rpi host so we need to manually set initial states
		// not ideal as a pullup really isn't the same thing but it works for most use cases

		if err != nil {
			log.Println("Failed to set pullup on " + pinId + " setting high state instead : " + err.Error())
			h.broadcastSys <- []byte("Error setting pullup " + err.Error())
			// we failed to set pullup, so lets set initial state high instead
			err = pin.Write(1)
			state = 1
			if err != nil {
				log.Println("Failed to write to pin on " + pinId + " : " + err.Error())
				h.broadcastSys <- []byte("Error writing to pin " + err.Error())
				return	
			}
		}
	} else if pullup == Pull_Down {
		err = pin.PullDown()	

		if err != nil {

			log.Println("Failed to set pulldown on " + pinId + " setting low state instead : " + err.Error())
			h.broadcastSys <- []byte("Error setting pulldown " + err.Error())
			// we failed to set pullup, so lets set initial state high instead
			err = pin.Write(0)
			state = 1
			if err != nil {
				log.Println("Failed to write to pin on " + pinId + " : " + err.Error())
				h.broadcastSys <- []byte("Error writing to pin " + err.Error())
				return	
			}
		}
	} // otherwise leave floating
	log.Println("Storing pin state")
	// test to see if we already have a state for this pin
	existingPin, exists := pinStates[pinId]
	if exists {
		existingPin.Pin = pin
		existingPin.Name = name
		existingPin.Dir = dir
		existingPin.State = state
		existingPin.Pullup = pullup
		pinStates[pinId] = existingPin
	} else {
		pinStates[pinId] = PinState{pin, pinId, dir, state, pullup, name}
	}
	log.Println("Pin state " + pinId + " stored.")
	gpioPinStates()
}
func gpioSetPin(pinId string, val int) {
	if pinState, ok := pinStates[pinId]; ok {
		log.Println("Found pin " + pinId + " setting to ", val)
		err := pinState.Pin.Write(int(val))
		if err != nil {
			log.Println("Failed to write to pin on " + pinId + " : " + err.Error())
			h.broadcastSys <- []byte("Error writing to pin " + err.Error())
		}
		// update pin state value
		existingPin, exists := pinStates[pinId]
		if exists {
			existingPin.State = val
			pinStates[pinId] = existingPin
		}
	}
}

func gpioRemovePin(pinId string) {
	if pinState, ok := pinStates[pinId]; ok {
		log.Println("Found pin " + pinId + " to remove")
		pinState.Pin.Close()
		delete(pinStates,pinId)
		gpioPinStates()
	}
}