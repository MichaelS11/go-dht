package dht

import (
	"fmt"
	"periph.io/x/host/v3"
	"time"
)

// HostInit calls periph.io host.Init(). This needs to be done before DHT can be used.
func HostInit() error {
	_, err := host.Init()
	return err
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
			temperature = float64(temperatureInt)*9.0/50.0 + 32.0
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
		temperature = float64(temperatureInt)*9.0/5.0 + 32.0
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

// ReadBackground it meant to be run in the background, run as a Goroutine.
// sleepDuration is how long it will try to sleep between reads.
// If there is ongoing read errors there will be no notice except that the values will not be updated.
// Will continue to read sensor until stop is closed.
// After it has been stopped, the stopped chan will be closed.
// Will panic if humidity, temperature, or stop are nil.
func (dht *DHT) ReadBackground(humidity *float64, temperature *float64, sleepDuration time.Duration, stop chan struct{}, stopped chan struct{}) {
	var humidityTemp float64
	var temperatureTemp float64
	var err error
	var startTime time.Time

Loop:
	for {
		startTime = time.Now()
		humidityTemp, temperatureTemp, err = dht.Read()
		if err == nil {
			// no read error, save result
			*humidity = humidityTemp
			*temperature = temperatureTemp
			// wait for sleepDuration or stop
			select {
			case <-time.After(sleepDuration - time.Since(startTime)):
			case <-stop:
				break Loop
			}
		} else {
			// read error, just check for stop
			select {
			case <-stop:
				break Loop
			default:
			}
		}
	}

	close(stopped)
}
