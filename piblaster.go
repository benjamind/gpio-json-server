package main

import (
  "os"
  "fmt"
  //"bytes"
  "strconv"
)

type Blaster struct {
  Pins map[int64]float64
}

func check(e error) {
  if e != nil {
    panic(e)
  }
}

var b = Blaster{
  Pins: make(map[int64]float64),
}

func (b *Blaster) Apply(pin int64, value float64) {
  f, err := os.Create("/dev/pi-blaster")
  check(err)
  defer f.Close()

  // ensure set value > 0, < 1
  if value > 1.0 {
    fmt.Printf("Request value exceeds 1, setting to 1\n")
    value = 1.0
  } else if value < 0.0 {
    fmt.Printf("Requested value below 0, setting to 0\n")
    value = 0.0
  }

  var toVal string
  toVal = strconv.FormatFloat(value, 'f', 2, 64)
  n1, err := f.WriteString(strconv.FormatInt(pin, 10) + "=" + toVal + "\n")
  b.Pins[pin] = value
  fmt.Printf("wrote %d bytes (%d = %s)\n", n1, pin, toVal)
  f.Sync()
}