package main

import (
	"log"
	"encoding/json"
	"strconv"
	"strings"
)

type hub struct {
	// Registered connections.
	connections map[*connection]bool

	// Inbound messages from the connections.
	broadcast chan []byte

	// Inbound messages from the system
	broadcastSys chan []byte

	// Register requests from the connections.
	register chan *connection

	// Unregister requests from connections.
	unregister chan *connection

	gpio *GPIO
}

var h = hub{
	broadcast:    make(chan []byte),
	broadcastSys: make(chan []byte),
	register:     make(chan *connection),
	unregister:   make(chan *connection),
	connections:  make(map[*connection]bool),
}

func (h *hub) run(gpio *GPIO) {
	h.gpio = gpio
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
			// send supported commands
			c.send <- []byte("{\"Version\" : \"" + version + "\"} ")
			c.send <- []byte("{\"Commands\" : [ \"gethost\", \"getpinmap\", \"getpinstates\", \"initpin\", \"setpin\", \"removepin\" ]} ")
		case c := <-h.unregister:
			delete(h.connections, c)
			// put close in func cuz it was creating panics and want
			// to isolate
			func() {
				// this method can panic if websocket gets disconnected
				// from users browser and we see we need to unregister a couple
				// of times, i.e. perhaps from incoming data from serial triggering
				// an unregister. (NOT 100% sure why seeing c.send be closed twice here)
				defer func() {
					if e := recover(); e != nil {
						log.Println("Got panic: ", e)
					}
				}()
				close(c.send)
			}()
		case m := <-h.broadcast:
			/*log.Print("Got a broadcast")
			log.Print(m)
			log.Print(len(m))*/
			if len(m) > 0 {
				/*log.Print(string(m))
				log.Print(h.broadcast)*/
				h.checkCmd(m)
				//log.Print("-----")

				for c := range h.connections {
					select {
					case c.send <- m:
						/*log.Print("did broadcast to ")
						log.Print(c.ws.RemoteAddr())*/
						//c.send <- []byte("hello world")
					default:
						delete(h.connections, c)
						close(c.send)
						go c.ws.Close()
					}
				}
			}
		case m := <-h.broadcastSys:
			/*log.Printf("Got a system broadcast: %v\n", string(m))
			log.Print(string(m))
			log.Print("-----")*/

			for c := range h.connections {
				select {
				case c.send <- m:
					/*log.Print("did broadcast to ")
					log.Print(c.ws.RemoteAddr())*/
					//c.send <- []byte("hello world")
				default:
					delete(h.connections, c)
					close(c.send)
					go c.ws.Close()
				}
			}
		}
	}
}

func (h *hub) sendErr(msg string) {
	msgMap := map[string]string {"error": msg}
	log.Println("Error: " + msg)
	bytes, err := json.Marshal(msgMap)
	if err!=nil {
		log.Println("Failed to marshal data!")
		return
	}
	h.broadcastSys <- bytes
}

func (h *hub) sendMsg(name string, msg interface{}) {
	msgMap := make(map[string] interface{})
	msgMap[name] = msg
	//log.Println("Sent: " + name)
	bytes, err := json.Marshal(msgMap)
	if err!=nil {
		log.Println("Failed to marshal data!")
		return
	}
	h.broadcastSys <- bytes
}
func (h *hub) checkCmd(m []byte) {
	//log.Println("Inside checkCmd")
	s := string(m[:])
	s = strings.Replace(s, "\n", "", -1)

	sl := strings.ToLower(s)
	log.Print(sl)

	if strings.HasPrefix(sl, "gethost") {
		hostname,err := h.gpio.Host()
		if err!=nil {
			go h.sendErr(err.Error())
		}
		go h.sendMsg("Host", hostname)

	} else if strings.HasPrefix(sl, "getpinmap") {
		pinMap,err := h.gpio.PinMap()
		if err!=nil {
			go h.sendErr(err.Error())
		}
		go h.sendMsg("PinMap", pinMap)
	} else if strings.HasPrefix(sl, "getpinstates") {
		pinStates,err := h.gpio.PinStates()
		if err!=nil {
			go h.sendErr(err.Error())
		}
		go h.sendMsg("PinStates", pinStates)

	} else if strings.HasPrefix(sl, "initpin") {
		// format : setpin pinId dir pullup
		args := strings.Split(s, " ")
		if len(args) < 4 {
			go h.sendErr("You did not specify a pin and a direction [0|1|low|high] and a name")
			return
		}
		if len(args[1]) < 1 {
			go h.sendErr("You did not specify a pin")
			return
		}
		pin := args[1]
		dirStr := args[2]
		name := args[4]
		dir := In
		switch {
			case dirStr == "1" || dirStr == "out" || dirStr == "output":
				dir = Out
			case dirStr == "0" || dirStr == "in" || dirStr == "input":
				dir = In
			case dirStr == "pwm":
				dir = PWM
		}
		pullup := Pull_None
		switch {
			case args[3] == "1" || args[3] == "up":
				pullup = Pull_Up
			case args[3] == "0" || args[3] == "down":
				pullup = Pull_Down
		}
		err := h.gpio.PinInit(pin, dir, pullup, name)
		if err != nil {
			go h.sendErr(err.Error())
		}
	} else if strings.HasPrefix(sl, "removepin") {
		// format : removepin pinId
		args := strings.Split(s, " ")
		if len(args) < 2 {
			go h.sendErr("You did not specify a pin id")
			return
		}
		err := h.gpio.PinRemove(args[1])
		if err != nil {
			go h.sendErr(err.Error())
		}
	} else if strings.HasPrefix(sl, "setpin") {
		// format : setpin pinId high/low/1/0
		args := strings.Split(s, " ")
		if len(args) < 3 {
			go h.sendErr("You did not specify a pin and a state [0|1|low|high]")
			return
		}
		if len(args[1]) < 1 {
			go h.sendErr("You did not specify a pin")
			return
		}
		pin := args[1]
		stateStr := args[2]
		state := 0
		switch {
			case stateStr == "1" || stateStr == "high":
				state = 1
			case stateStr == "0" || stateStr == "low":
				state = 0
			default:
				// assume its a pwm value...if it converts to integer in 0-255 range
				s, err := strconv.Atoi(stateStr)
				if err != nil {
					go h.sendErr("Invalid value, must be between 0 and 255 : " + stateStr)
					return
				}
				if s < 0 || s > 255 {
					go h.sendErr("Invalid value, must be between 0 and 255 : " + stateStr)
					return
				}
				state = s
		}
		
		err := h.gpio.PinSet(pin, byte(state))
		if err != nil {
			go h.sendErr(err.Error())
		}
	}

	//log.Println("Done with checkCmd")
}