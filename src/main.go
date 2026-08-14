package main

import (
	"strings"
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// battery datat structures
type Bat struct {
	IsCharging bool
	Charge uint8
	Timeh uint8
	Timem uint8
}
var bat_lock = sync.Mutex{}
var bat_data = Bat{}

// modkey data structures
type ModKeys struct {
    Shift bool `json:"shift"`
    Ctrl  bool `json:"ctrl"`
    Meta   bool `json:"meta"`
    Alt   bool `json:"alt"`
}
var modkey_lock = sync.Mutex{}
var modkey_data = ModKeys{}


func readIntFromFs(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	value, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	return value
}

func main() {
	go modkey_update()
	for true {
		time := time.Date()
				
	}

}

func bat_update() {
	for true {
		capacity := readIntFromFs("/sys/class/power_supply/BAT0/capacity")
		status, err := os.ReadFile("/sys/class/power_supply/BAT0/status")
		if err != nil {
			log.Fatal("Error while reading status")
		}
		energy_now := readIntFromFs("/sys/class/power_supply/BAT0/energy_now")
		power_now := readIntFromFs("/sys/class/power_supply/BAT0/power_now")

		var charging = false
		if string(status) == "Charging" {
			charging = true
		}
		remaining := time.Duration(float64(energy_now) / float64(power_now) * float64(time.Hour))

		hours := uint8(remaining.Hours())
		minutes := uint8(remaining.Minutes()) % 60


		// update
		bat_lock.Lock()
		bat_data.IsCharging = charging
		bat_data.Charge = uint8(capacity)
		bat_data.Timeh = hours
		bat_data.Timem = minutes
		bat_lock.Unlock()

		time.Sleep(1000 * time.Millisecond) // update only every second
	}
}

func modkey_update() {
	var conn, err = net.Dial("unix", "/tmp/modifier.sock")
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		modkey_lock.Lock()
		err := json.Unmarshal([]byte(line), &modkey_data)
		if err != nil {
			log.Fatal("Something went wrong while parsing json")
		} else {
			fmt.Print(modkey_data)
		}
		modkey_lock.Unlock()
	}
	if err := scanner.Err(); err != nil {
		log.Fatal("Scan Error: ", err)	
	}
}
