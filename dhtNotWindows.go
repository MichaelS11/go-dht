//go:build !windows
// +build !windows

package dht

import (
	"fmt"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"runtime/debug"
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

	// get pin
	dht.pin = gpioreg.ByName(pinName)
	if dht.pin == nil {
		return nil, fmt.Errorf("pin is nill")
	}

	// set pin to high so ready for first read
	err := dht.pin.Out(gpio.High)
	if err != nil {
		return nil, fmt.Errorf("pin out high error: %v", err)
	}

	// set lastRead a second before to give the pin a second to warm up
	dht.lastRead = time.Now().Add(-1 * time.Second)

	return dht, nil
}

// readBits will get the bits for humidity and temperature
func (dht *DHT) readBits() ([]int, error) {
	// create variables ahead of time before critical timing part
	var i int
	var startTime time.Time
	var levelPrevious gpio.Level
	var level gpio.Level
	levels := make([]gpio.Level, 0, 84)
	durations := make([]time.Duration, 0, 84)

	// set lastRead so do not read more than once every 2 seconds
	dht.lastRead = time.Now()

	// disable garbage collection during critical timing part
	gcPercent := debug.SetGCPercent(-1)

	// send start low
	err := dht.pin.Out(gpio.Low)
	if err != nil {
		dht.pin.Out(gpio.High)
		return nil, fmt.Errorf("pin out low error: %v", err)
	}
	time.Sleep(time.Millisecond)

	// send start high
	err = dht.pin.In(gpio.PullUp, gpio.NoEdge)
	if err != nil {
		dht.pin.Out(gpio.High)
		return nil, fmt.Errorf("pin in error: %v", err)
	}

	// read levels and durations with busy read
	// hope there is a better way in the future
	// tried to use WaitForEdge but seems to miss edges and/or take too long to detect them
	// note that pin read takes around .2 microsecond (us) on Raspberry PI 3
	// note that 1000 microsecond (us) = 1 millisecond (ms)
	levelPrevious = dht.pin.Read()
	level = levelPrevious
	for i = 0; i < 84; i++ {
		startTime = time.Now()
		for levelPrevious == level && time.Since(startTime) < time.Millisecond {
			level = dht.pin.Read()
		}
		durations = append(durations, time.Since(startTime))
		levels = append(levels, levelPrevious)
		levelPrevious = level
	}

	// enable garbage collection, done with critical part
	debug.SetGCPercent(gcPercent)

	// set pin to high so ready for next time
	err = dht.pin.Out(gpio.High)
	if err != nil {
		return nil, fmt.Errorf("pin out high error: %v", err)
	}

	// get last low reading so know start of data
	var endNumber int
	for i = len(levels) - 1; ; i-- {
		if levels[i] == gpio.Low {
			endNumber = i
			break
		}
		if i < 80 {
			// not enough readings, i = 79 means endNumber is 78 or less
			return nil, fmt.Errorf("missing some readings - low level not found")
		}
	}
	startNumber := endNumber - 79

	// covert pulses into bits and check high levels
	bits := make([]int, 40)
	index := 0
	for i = startNumber; i < endNumber; i += 2 {
		// check high levels
		if levels[i] != gpio.High {
			return nil, fmt.Errorf("missing some readings - level not high")
		}
		// high should not be longer then 90 microseconds
		if durations[i] > 90*time.Microsecond {
			return nil, fmt.Errorf("missing some readings - high level duration too long: %v", durations[i])
		}
		// bit is 0 if less than or equal to 30 microseconds
		if durations[i] > 30*time.Microsecond {
			// bit is 1 if more than 30 microseconds
			bits[index] = 1
		}
		index++
	}

	// check low levels
	for i = startNumber + 1; i < endNumber+1; i += 2 {
		// check low levels
		if levels[i] != gpio.Low {
			return nil, fmt.Errorf("missing some readings - level not low")
		}
		// low should not be longer then 70 microseconds
		if durations[i] > 70*time.Microsecond {
			return nil, fmt.Errorf("missing some readings - low level duration too long: %v", durations[i])
		}
		// low should not be shorter than 35 microseconds
		if durations[i] < 35*time.Microsecond {
			return nil, fmt.Errorf("missing some readings - low level duration too short: %v", durations[i])
		}
	}

	return bits, nil
}
