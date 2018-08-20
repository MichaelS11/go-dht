# Go DHT22 / AM2302 / DHT11 interface

Golang DHT22 / AM2302 / DHT11 interface using periph.io driver

[![GoDoc Reference](https://godoc.org/github.com/MichaelS11/go-dht?status.svg)](http://godoc.org/github.com/MichaelS11/go-dht)
[![Go Report Card](https://goreportcard.com/badge/github.com/MichaelS11/go-dht)](https://goreportcard.com/report/github.com/MichaelS11/go-dht)


## Please note

Please make sure to setup your DHT22 / AM2302 / DHT11 correctly. Do a search on the internet to find guide. Here is an example of a guide:

https://learn.adafruit.com/dht/connecting-to-a-dhtxx-sensor

The examples below are from using a Raspberry Pi 3 with GPIO 19 for the pin. Your setup may be different, if so, your pin names would need to change in each example.

Side note, in my testing the sensor has a fairly high level of read errors, suggest using ReadRetry or ReadBackground rather then just Read.

Tested on Raspberry Pi 3 with AM2302. Please open an issue if there are any issues.


## Get

go get github.com/MichaelS11/go-dht


## ReadRetry example

```go
package main

import (
	"fmt"

	"github.com/MichaelS11/go-dht"
)

func main() {
	err := dht.HostInit()
	if err != nil {
		fmt.Println("HostInit error:", err)
		return
	}

	dht, err := dht.NewDHT("GPIO19", dht.Fahrenheit, "")
	if err != nil {
		fmt.Println("NewDHT error:", err)
		return
	}

	humidity, temperature, err := dht.ReadRetry(11)
	if err != nil {
		fmt.Println("Read error:", err)
		return
	}

	fmt.Printf("humidity: %v\n", humidity)
	fmt.Printf("temperature: %v\n", temperature)
}

```


## ReadBackground example

```go
package main

import (
	"fmt"
	"time"

	"github.com/MichaelS11/go-dht"
)

func main() {
	err := dht.HostInit()
	if err != nil {
		fmt.Println("HostInit error:", err)
		return
	}

	dht, err := dht.NewDHT("GPIO19", dht.Fahrenheit, "")
	if err != nil {
		fmt.Println("NewDHT error:", err)
		return
	}

	stop := make(chan struct{})
	stopped := make(chan struct{})
	var humidity float64
	var temperature float64

	// get sensor reading every 20 seconds in background
	go dht.ReadBackground(&humidity, &temperature, 20*time.Second, stop, stopped)

	// should have at least read the sensor twice after 30 seconds
	time.Sleep(30 * time.Second)

	fmt.Printf("humidity: %v\n", humidity)
	fmt.Printf("temperature: %v\n", temperature)

	// when done reading and to stop ReadBackground, close stop channel
	close(stop)

	// can check stopped channel to get when ReadBackground has stopped
	<-stopped
}
```


## Read example

```go
package main

import (
	"fmt"

	"github.com/MichaelS11/go-dht"
)

func main() {
	err := dht.HostInit()
	if err != nil {
		fmt.Println("HostInit error:", err)
		return
	}

	dht, err := dht.NewDHT("GPIO19", dht.Fahrenheit, "")
	if err != nil {
		fmt.Println("NewDHT error:", err)
		return
	}

	humidity, temperature, err := dht.Read()
	if err != nil {
		fmt.Println("Read error:", err)
		return
	}

	fmt.Printf("humidity: %v\n", humidity)
	fmt.Printf("temperature: %v\n", temperature)
}

```
