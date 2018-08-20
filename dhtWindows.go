// +build windows

package dht

import (
	"strings"
	"time"
)

// NewDHT to create a new DHT struct.
// sensorType is dht11 for DHT11, anything else for AM2302 / DHT22.
func NewDHT(pinName string, temperatureUnit TemperatureUnit, sensorType string) (*DHT, error) {
	dht := &DHT{temperatureUnit: temperatureUnit}

	// set sensorType
	sensorType = strings.ToLower(sensorType)
	if sensorType == "dht11" {
		dht.sensorType = "dht11"
	}

	// set lastRead a second before to give the pin a second to warm up
	dht.lastRead = time.Now().Add(-1 * time.Second)

	return dht, nil
}

// readBits will get the bits for humidity and temperature
func (dht *DHT) readBits() ([]int, error) {
	// set lastRead so do not read more than once every 2 seconds
	dht.lastRead = time.Now()

	bits := make([]int, 40)

	return bits, nil
}
