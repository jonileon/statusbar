package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/itchyny/volume-go"
)

const inactive_color = "#555555"
const active_color = "#ffffff"
const output_format = "<b><span foreground=\"%v\">ctrl</span> <span foreground=\"%v\">shift</span> <span foreground=\"%v\">alt</span> <span foreground=\"%v\" >super</span> │ power-prof: <span foreground=\"#ffbb55\">%v</span> │ wifi: <span foreground=\"#8888dd\">HaDiFunk</span> │ %v <span foreground=\"#ff5555\">%v%%</span> │  [BAT] %v%% <span foreground=\"#ffffbd\">(%02d:%02d)</span> │ %v %v-%02d-%02d <span foreground=\"#ffff7b\">%02d:%02d:%02d</span></b>\n"

// battery datat structures
type Bat struct {
	IsCharging bool
	Charge uint8
	Timeh  uint8
	Timem  uint8
}
var bat_lock = sync.Mutex{}
var bat_data = Bat{}

// modkey data structures
type ModKeys struct {
    Shift bool `json:"shift"`
    Ctrl  bool `json:"ctrl"`
    Meta  bool `json:"meta"`
    Alt   bool `json:"alt"`
}
var modkey_lock = sync.Mutex{}
var modkey_data = ModKeys{}

// volume data structures
type VolumeInfo struct {
	Muted bool
	Volume uint8
}
var volume_lock = sync.Mutex{}
var volume_data = VolumeInfo{}

// powerprofile
var powerprofile = ""
var powerprofile_lock = sync.Mutex{}


func readIntFromFs(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatal("os fs: ", err)
	}
	value, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		log.Fatal("parse: ", err)
	}
	return value
}

func main() {
	go modkey_update()
	go bat_update()
	go volume_update()
	go powerprofile_update()
	for true {
		now := time.Now()
		year, month, day := now.Date()

		ctrl := inactive_color
		meta := inactive_color
		alt := inactive_color
		shift := inactive_color
		modkey_lock.Lock()
		if modkey_data.Ctrl {
			ctrl = active_color
		}
		if modkey_data.Meta {
			meta = active_color
		}
		if modkey_data.Alt {
			alt = active_color
		}
		if modkey_data.Shift {
			shift = active_color
		}
		modkey_lock.Unlock()

		vol_string := "[VOL]"
		volume_lock.Lock()
		vol := volume_data.Volume
		if volume_data.Muted {
			vol_string = "[MUT]"
		}
		volume_lock.Unlock()

		bat_lock.Lock()
		bat := bat_data.Charge
		h := bat_data.Timeh
		m := bat_data.Timem
		bat_lock.Unlock()

		powerprofile_lock.Lock()
		profile := powerprofile
		powerprofile_lock.Unlock()
		
		fmt.Printf(output_format,
			// modifier keys
			ctrl, shift, alt, meta,
			// powerprofile
			profile,
			// volume
			vol_string, vol,
			// battery
			bat, h, m,
			// time date
			now.Weekday().String(),
			year, int(month), day,
			now.Hour(), now.Minute(), now.Second(),
		)

		time.Sleep(500 * time.Millisecond)
	}

}

func bat_update() {
	for true {
		capacity := readIntFromFs("/sys/class/power_supply/BAT0/capacity")
		status, err := os.ReadFile("/sys/class/power_supply/BAT0/status")
		if err != nil {
			log.Fatal("Error while reading battery status")
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
		}		
		modkey_lock.Unlock()
	}
	if err := scanner.Err(); err != nil {
		log.Fatal("Scan Error: ", err)	
	}
}

func volume_update() {
	for true {
		muted, err := volume.GetMuted()
		if err != nil {
			log.Fatal("volume: ", err)
		}
		vol, err := volume.GetVolume()
		if err != nil {
			log.Fatal("volume: ", err)
		}

		// update
		volume_lock.Lock()
		volume_data.Muted = muted
		volume_data.Volume = uint8(vol)
		volume_lock.Unlock()
		
		time.Sleep(500 * time.Millisecond)
	}
}

func powerprofile_update() {
	for true {
		cmd := exec.Command("tuned-adm", "active")
		output, err := cmd.Output()
		if err != nil {
			log.Fatal("powerprofile: ", err)
		}
		parts := strings.Fields(string(output))
		profile := parts[len(parts) - 1]

		powerprofile_lock.Lock()
		powerprofile = profile
		powerprofile_lock.Unlock()

		time.Sleep(1000 * time.Millisecond)
	}
}
