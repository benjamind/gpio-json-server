GPIO JSON Server
================

A (very) rudimentary interface to provide GPIO control on a remote server via a simple interface.

This is primarily for use with the ChiliPeppr CNC control software (http://www.chilipeppr.com)

This is still very much alpha code, and was my first Go project, so its a bit messy. This will get cleaned up when I get some time.

It has thus far only been tested on a Raspberry Pi, but embd (the gpio library) supports Beaglebone Black too so in theory it should work there also.

Current implementation allows you to setup GPIO pin states and toggle them through a simple interface. Future features will include w1-therm monitoring for temperature monitoring, PWM control for LED lighting and other PWM uses, and a cleaner JSON based interface.

Installation
============

This requires GPIO port access which itself requires sudo, so run with
```
sudo ./gpio-json-server
```