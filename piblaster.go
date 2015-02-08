// +build linux,arm

package main

import (
	"os"
	"strconv"
	"errors"
	"log"
)

type BlasterPin struct {
	id int
	value float64
}

func InitBlaster() error {
	// check the file actually exists, throw error if not so Pi can bail out
	if _, err := os.Stat("/dev/pi-blaster"); os.IsNotExist(err) {
		return errors.New("/dev/pi-blaster does not exists, is pi-blaster correctly installed?")
	}
	return nil
}
func CloseBlaster() error {
	return nil
}
func NewBlasterPin(pinId int) BlasterPin {
	log.Println("Creating pi blaster pin on ", string(pinId))
	return BlasterPin {
		pinId,
		0.0,
	}
}
func (b *BlasterPin) Close() error {
	f, err := os.Create("/dev/pi-blaster")
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("release " + strconv.Itoa(b.id))
	if err != nil {
		return err
	}
	f.Sync()
	return nil
}
func (b *BlasterPin) Write(value byte) error {
	f, err := os.Create("/dev/pi-blaster")
	if err != nil {
		return err
	}
	defer f.Close()

	v := (float64(value)/255.0)
	if v > 1.0 {
		v = 1.0
	} else if v < 0.0 {
		v = 0.0
	}
	toVal := strconv.FormatFloat(v, 'f', 2, 64)
	msg := strconv.Itoa(b.id) + "=" + string(toVal)
	_, err = f.WriteString(msg + "\n")
	if err != nil {
		log.Println("PiBlaster: Failed to write :", err.Error())
		return err
	}
	b.value = v
	f.Sync()
	return nil
}