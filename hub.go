package main

import (
	"log"
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
}

var h = hub{
	broadcast:    make(chan []byte),
	broadcastSys: make(chan []byte),
	register:     make(chan *connection),
	unregister:   make(chan *connection),
	connections:  make(map[*connection]bool),
}

func (h *hub) run() {
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
			// send supported commands
			c.send <- []byte("{\"Version\" : \"" + version + "\"} ")
			c.send <- []byte("{\"Commands\" : [ \"gethost\" ]} ")
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
				checkCmd(m)
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

func sendErr(msg string) {
	log.Println("Error: " + msg)
	h.broadcastSys <- []byte(msg)
}

func checkCmd(m []byte) {
	//log.Println("Inside checkCmd")
	s := string(m[:])
	s = strings.Replace(s, "\n", "", -1)

	sl := strings.ToLower(s)
	log.Print(sl)

	if strings.HasPrefix(sl, "gethost") {
		go gpioHost()
	} else if strings.HasPrefix(sl, "getpinmap") {
		go gpioPinMap()
	} else if strings.HasPrefix(sl, "getpinstates") {
		go gpioPinStates()
	} else if strings.HasPrefix(sl, "setpwm") {
		args := strings.Split(s, " ")
		if len(args) < 3 {
			go sendErr("You did not specify a pin and a duty cycle")
			return
		}
		if len(args[1]) < 1 {
			go sendErr("You did not specify a pin")
			return
		}
		duty, err := strconv.Atoi(args[2])
		if err != nil {
			go sendErr("Error converting duty cycle to int : " + err.Error())
			return
		}
		pin := args[1]
		go gpioPWMPin(pin, byte(duty))
	} else if strings.HasPrefix(sl, "initpin") {
		// format : setpin pinId dir pullup
		args := strings.Split(sl, " ")
		if len(args) < 4 {
			go sendErr("You did not specify a pin and a direction [0|1|low|high] and a name")
			return
		}
		if len(args[1]) < 1 {
			go sendErr("You did not specify a pin")
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
		}
		pullup := Pull_None
		switch {
			case args[3] == "1" || args[3] == "up":
				pullup = Pull_Up
			case args[3] == "0" || args[3] == "down":
				pullup = Pull_Down
		}
		
		
		go gpioInitPin(pin, dir, pullup, name)
	} else if strings.HasPrefix(sl, "removepin") {
		// format : removepin pinId
		args := strings.Split(sl, " ")
		if len(args) < 2 {
			go sendErr("You did not specify a pin id")
			return
		}
		go gpioRemovePin(args[1])
	} else if strings.HasPrefix(sl, "setpin") {
		// format : setpin pinId high/low/1/0
		args := strings.Split(sl, " ")
		if len(args) < 3 {
			go sendErr("You did not specify a pin and a state [0|1|low|high]")
			return
		}
		if len(args[1]) < 1 {
			go sendErr("You did not specify a pin")
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
		}
		
		go gpioSetPin(pin, state)
	}

	//log.Println("Done with checkCmd")
}