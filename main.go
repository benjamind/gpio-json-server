package main
	
import (
	"flag"
	"log"
	"net/http"
	"net"
	"errors"
	"os"
	"os/signal"
	"text/template"
	"io/ioutil"
	"encoding/json"
)

var (
	version			= "1.1"
	versionFloat	= float32(1.1)
	addr			= flag.String("addr", ":8888", "http service address")
)

type Direction int
type PullUp int

type PinState struct {
	Pin interface{} `json:"-"`
	PinId string
	Dir Direction
	State byte
	Pullup PullUp
	Name string
}

type PinDef struct {
	ID string
	Aliases []string
	Capabilities []string
	DigitalLogical int
	AnalogLogical int
}

const STATE_FILE = "pinstates.json"

const (
	In Direction = 0
	Out Direction = 1
	PWM Direction = 2

	Pull_None PullUp = 0
	Pull_Up PullUp = 1
	Pull_Down PullUp = 2
)

type GPIOInterface interface {
	Init(chan PinState, chan PinState, chan string, map[string] PinState) error
	Close() error
	PinMap() ([]PinDef, error)
	Host() (string, error)
	PinStates() (map[string] PinState, error)
	PinInit(string, Direction, PullUp, string) error
	PinSet(string, byte) error
	PinRemove(string) error
}

type NullWriter int

func (NullWriter) Write([]byte) (int, error) { return 0, nil }

func homeHandler(c http.ResponseWriter, req *http.Request) {
	homeTemplate.Execute(c, req.Host)
}

func cleanup(gpio GPIOInterface) {
	pinStates, err := gpio.PinStates()
	if err != nil {
		log.Println("Error getting pinstates on cleanup: " + err.Error())
	} else {
		data, err := json.Marshal(pinStates)
		if err!=nil {
			log.Println("Error marshalling pin states : " + err.Error())
		}
		ioutil.WriteFile(STATE_FILE, data, 0644)
	}
	gpio.Close()                              
	os.Exit(1)
}
func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	flag.Parse()
	f := flag.Lookup("addr")
	log.Println("Version:" + version)

	ip, err := externalIP()
	if err != nil {
		log.Fatalln(err)
	}

	log.Print("Started server and websocket on " + ip + "" + f.Value.String())

	log.Println("The GPIO JSON Server is now running.")
	log.Println("If you are using ChiliPeppr, you may go back to it and connect to this server using the GPIO widget.")

	/*if !*verbose {
		log.Println("You can enter verbose mode to see all logging by starting with the -v command line switch.")
		log.SetOutput(new(NullWriter)) //route all logging to nullwriter
	}*/

	gpio := new(GPIO)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func(){
		for sig := range c {
			// sig is a ^C, handle it  
			log.Printf("captured %v, cleaning up gpio and exiting..", sig) 
			cleanup(gpio)
		}
	}()
	defer cleanup(gpio)

	stateChanged := make(chan PinState)
	pinRemoved := make(chan string)
	pinAdded := make(chan PinState)
	go func() {
		for {
			// start listening on stateChanged and pinRemoved channels and update hub as appropriate
			select {
			case pinState := <- stateChanged:
				go h.sendMsg("PinState",pinState)
			case pinName := <- pinRemoved:
				go h.sendMsg("PinRemoved",pinName)
			case pinState := <- pinAdded:
				go h.sendMsg("PinAdded",pinState)
			}
		}
	}()
	// launch the hub routine which is the singleton for the websocket server
	go h.run(gpio)

	pinStates := make(map[string] PinState)

	// read existing pin states
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
	}

	gpio.Init(stateChanged, pinAdded, pinRemoved, pinStates)

	
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/ws", wsHandler)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal("Error ListenAndServe:", err)
	}
}

func externalIP() (string, error) {
	//log.Println("Getting external IP")
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Println("Got err getting external IP addr")
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			//log.Println("Iface down")
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			//log.Println("Loopback")
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			log.Println("Got err on iface.Addrs()")
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				//log.Println("Ip was nil or loopback")
				continue
			}
			ip = ip.To4()
			if ip == nil {
				//log.Println("Was not ipv4 addr")
				continue // not an ipv4 address
			}
			//log.Println("IP is ", ip.String())
			return ip.String(), nil
		}
	}
	return "", errors.New("are you connected to the network?")
}

var homeTemplate = template.Must(template.New("home").Parse(homeTemplateHtml))

// If you navigate to this server's homepage, you'll get this HTML
// so you can directly interact with the serial port server
const homeTemplateHtml = `<!DOCTYPE html>
<html>
<head>
<title>Serial Port Example</title>
<script type="text/javascript" src="http://ajax.googleapis.com/ajax/libs/jquery/1.4.2/jquery.min.js"></script>
<script type="text/javascript">
	$(function() {
	var conn;
	var msg = $("#msg");
	var log = $("#log");
	function appendLog(msg) {
		var d = log[0]
		var doScroll = d.scrollTop == d.scrollHeight - d.clientHeight;
		msg.appendTo(log)
		if (doScroll) {
			d.scrollTop = d.scrollHeight - d.clientHeight;
		}
	}
	$("#form").submit(function() {
		if (!conn) {
			return false;
		}
		if (!msg.val()) {
			return false;
		}
		conn.send(msg.val() + "\n");
		msg.val("");
		return false
	});
	if (window["WebSocket"]) {
		conn = new WebSocket("ws://{{$}}/ws");
		conn.onclose = function(evt) {
			appendLog($("<div><b>Connection closed.</b></div>"))
		}
		conn.onmessage = function(evt) {
			appendLog($("<div/>").text(evt.data))
		}
	} else {
		appendLog($("<div><b>Your browser does not support WebSockets.</b></div>"))
	}
	});
</script>
<style type="text/css">
html {
	overflow: hidden;
}
body {
	overflow: hidden;
	padding: 0;
	margin: 0;
	width: 100%;
	height: 100%;
	background: gray;
}
#log {
	background: white;
	margin: 0;
	padding: 0.5em 0.5em 0.5em 0.5em;
	position: absolute;
	top: 0.5em;
	left: 0.5em;
	right: 0.5em;
	bottom: 3em;
	overflow: auto;
}
#form {
	padding: 0 0.5em 0 0.5em;
	margin: 0;
	position: absolute;
	bottom: 1em;
	left: 0px;
	width: 100%;
	overflow: hidden;
}
</style>
</head>
<body>
<div id="log"></div>
<form id="form">
	<input type="submit" value="Send" />
	<input type="text" id="msg" size="64"/>
</form>
</body>
</html>
`