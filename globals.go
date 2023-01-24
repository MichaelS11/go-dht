package dht

import (
	"periph.io/x/conn/v3/gpio"
	"time"
)

// TemperatureUnit is the temperature unit wanted, either Celsius or Fahrenheit
type TemperatureUnit int

const (
	// Celsius temperature unit
	Celsius TemperatureUnit = iota
	// Fahrenheit temperature unit
	Fahrenheit
)

// DHT struct to interface with the sensor.
// Call NewDHT to create a new one.
type DHT struct {
	pin             gpio.PinIO
	temperatureUnit TemperatureUnit
	sensorType      string
	numErrors       int
	lastRead        time.Time
}
