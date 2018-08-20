package dht

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/host"
)

// HostInit calls periph.io host.Init(). This needs to be done before DHT can be used.
func HostInit() error {
	_, err := host.Init()
	return err
}

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

// Read reads the sensor once, returing humidity and temperature, or an error.
// Note that Read will sleep for at least 2 seconds between last call.
// Each reads error adds a half second to sleep time to max of 30 seconds.
func (dht *DHT) Read() (humidity float64, temperature float64, err error) {
	// set sleepTime
	var sleepTime time.Duration
	if dht.numErrors < 57 {
		sleepTime = (2 * time.Second) + (time.Duration(dht.numErrors) * 500 * time.Millisecond)
	} else {
		// sleep max of 30 seconds
		sleepTime = 30 * time.Second
	}
	sleepTime -= time.Since(dht.lastRead)

	// sleep between 2 and 30 seconds
	time.Sleep(sleepTime)

	// read bits from sensor
	var bits []int
	bits, err = dht.readBits()
	if err != nil {
		return
	}

	// covert bits to humidity and temperature
	humidity, temperature, err = dht.bitsToValues(bits)

	return
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
		// low should not be shorter then 35 microseconds
		if durations[i] < 35*time.Microsecond {
			return nil, fmt.Errorf("missing some readings - low level duration too short: %v", durations[i])
		}
	}

	return bits, nil
}

// bitsToValues will convert the bits into humidity and temperature values
func (dht *DHT) bitsToValues(bits []int) (humidity float64, temperature float64, err error) {
	var sum8 uint8
	var sumTotal uint8
	var checkSum uint8
	var i int
	var humidityInt int
	var temperatureInt int

	// get humidityInt value
	for i = 0; i < 16; i++ {
		humidityInt = humidityInt << 1
		humidityInt += bits[i]
		// sum 8 bits for checkSum
		sum8 = sum8 << 1
		sum8 += uint8(bits[i])
		if i == 7 || i == 15 {
			// got 8 bits, add to sumTotal for checkSum
			sumTotal += sum8
			sum8 = 0
		}
	}

	// get temperatureInt value
	for i = 16; i < 32; i++ {
		temperatureInt = temperatureInt << 1
		temperatureInt += bits[i]
		// sum 8 bits for checkSum
		sum8 = sum8 << 1
		sum8 += uint8(bits[i])
		if i == 23 || i == 31 {
			// got 8 bits, add to sumTotal for checkSum
			sumTotal += sum8
			sum8 = 0
		}
	}
	// if high 16 bit is set, value is negtive
	// 1000000000000000 = 0x8000
	if (temperatureInt & 0x8000) > 0 {
		// flip bits 16 and lower to get negtive number for int
		// 1111111111111111 = 0xffff
		temperatureInt |= ^0xffff
	}

	// get checkSum value
	for i = 32; i < 40; i++ {
		checkSum = checkSum << 1
		checkSum += uint8(bits[i])
	}

	if dht.sensorType != "dht11" {
		// humidity is between 0 % to 100 %
		if humidityInt < 0 || humidityInt > 1000 {
			err = fmt.Errorf("bad data - humidity: %v", humidityInt)
			return
		}
		// temperature between -40 C to 80 C
		if temperatureInt < -400 || temperatureInt > 800 {
			err = fmt.Errorf("bad data - temperature: %v", temperatureInt)
			return
		}
		// check checkSum
		if checkSum != sumTotal {
			err = fmt.Errorf("bad data - check sum fail")
		}

		humidity = float64(humidityInt) / 10.0
		if dht.temperatureUnit == Celsius {
			temperature = float64(temperatureInt) / 10.0
		} else {
			temperature = float64(temperatureInt)/10.0*9.0/5000.0 + 32.0
		}

		return
	}

	// humidity is between 0 % to 100 %
	if humidityInt < 0 || humidityInt > 100 {
		err = fmt.Errorf("bad data - humidity: %v", humidityInt)
		return
	}
	// temperature between 0 C to 50 C
	if temperatureInt < 0 || temperatureInt > 50 {
		err = fmt.Errorf("bad data - temperature: %v", temperatureInt)
		return
	}
	// check checkSum
	if checkSum != sumTotal {
		err = fmt.Errorf("bad data - check sum fail")
	}

	humidity = float64(humidityInt)
	if dht.temperatureUnit == Celsius {
		temperature = float64(temperatureInt)
	} else {
		temperature = float64(temperatureInt)*9.0/5000.0 + 32.0
	}

	return
}

// ReadRetry will call Read until there is no errors or the maxRetries is hit.
// Suggest maxRetries to be set around 11.
func (dht *DHT) ReadRetry(maxRetries int) (humidity float64, temperature float64, err error) {
	for i := 0; i < maxRetries; i++ {
		humidity, temperature, err = dht.Read()
		if err == nil {
			return
		}
	}
	return
}

// ReadBackground it means to run in the background, run as a Goroutine.
// sleepDuration is how long it will try to sleep between reads.
// If there is ongoing read errors there will be no notice except that the values will not be updated.
// Will continue to read sensor until stop is set to true.
// After it has been stopped, the stopped chan will be closed.
// Will panic if humidity, temperature, or stop are nil.
func (dht *DHT) ReadBackground(humidity *float64, temperature *float64, sleepDuration time.Duration, stop chan struct{}, stopped chan struct{}) {
	var humidityTemp float64
	var temperatureTemp float64
	var err error
	startTime := time.Now().Add(-sleepDuration)

Loop:
	for {
		if err == nil {
			// no read error, wait for sleepDuration or stop
			select {
			case <-time.After(sleepDuration - time.Since(startTime)):
			case <-stop:
				break Loop
			}
		} else {
			// read error, just check wait for stop
			select {
			case <-stop:
				break Loop
			default:
			}
		}

		startTime = time.Now()
		humidityTemp, temperatureTemp, err = dht.Read()
		if err != nil {
			continue
		}

		*humidity = humidityTemp
		*temperature = temperatureTemp
	}

	close(stopped)
}
