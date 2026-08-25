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

const inactive_color     = "#555555"
const active_color       = "#c5c9c5"
const color_green        = "#55bb55"
const color_blue         = "#8888dd"
const color_red          = "#ff5555"
const color_orange       = "#ffbb55"
const color_light_yellow = "#ffffbd"
const color_yellow       = "#ffff7b"
const color_light_blue   = "#addddd"

const output_format = "<b><span foreground=\"%v\">ctrl</span> <span foreground=\"%v\">shift</span> <span foreground=\"%v\">alt</span> <span foreground=\"%v\" >meta</span> │" + // modkeys
" power-prof: <span foreground=\"" + color_orange + "\">%v</span> │" + //powerprofile
" wifi: <span foreground=\"%v\">%v</span> │" + // wifi
" %v <span foreground=\"%v\">%v%%</span> │" + // audio
" <span foreground=\"%v\">%v</span> <span foreground=\"%v\">%v%%</span> <span foreground=\"" + color_light_yellow + "\">%v</span> │" + // battery
" %v %v-%02d-%02d <span foreground=\"" + color_yellow + "\">%02d:%02d:%02d</span></b>\n" // date time

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

// wifi
type Wifi struct {
	active bool
	name string
}
var wifi_lock = sync.Mutex{}
var wifi_data = Wifi{}

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
	go wifi_update()
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

		vol_color := color_light_blue
		vol_string := "[VOL]"
		volume_lock.Lock()
		vol := volume_data.Volume
		if volume_data.Muted {
			vol_string = "[MUT]"
			vol_color = color_red
		}
		volume_lock.Unlock()
		if vol == 0 {
			vol_color = color_red
		}

		wifi_lock.Lock()
		wifi_color := color_blue
		wifi_name := wifi_data.name
		if !wifi_data.active {
			wifi_name = "none"
			wifi_color = active_color
		}
		wifi_lock.Unlock()

		bat_string := " [BAT]"
		bat_string_color := active_color
		bat_lock.Lock()
		rem_string := "(--:--)"
		bat := bat_data.Charge
		if !bat_data.IsCharging {
			rem_string = fmt.Sprintf("(%02d:%02d)", rune(bat_data.Timeh), rune(bat_data.Timem))
		} else {
			bat_string = "+[BAT]"
			bat_string_color = color_green
		}
		bat_color := active_color
		bat_lock.Unlock()
		if bat >= 70 {
			bat_color = color_green
		} else if bat <= 20 {
			bat_color = color_red
		} else if bat <= 40 {
			bat_color = color_orange
		}

		powerprofile_lock.Lock()
		profile := powerprofile
		powerprofile_lock.Unlock()
		
		fmt.Printf(output_format,
			// modifier keys
			ctrl, shift, alt, meta,
			// powerprofile
			profile,
			//wifi
			wifi_color, wifi_name,
			// volume
			vol_string, vol_color, vol,
			// battery
			bat_string_color, bat_string, bat_color, bat, rem_string,
			// time date
			now.Weekday().String()[:3],
			year, int(month), day,
			now.Hour(), now.Minute(), now.Second(),
		)

		time.Sleep(250 * time.Millisecond)
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
		if strings.TrimSpace(string(status)) == "Charging" {
			charging = true
		} else {
			charging = false
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
		// volume control may be uninitialized on startup, so errors can be 'ignored'
		if err != nil {
			println("volume: ", err)
			continue
		}
		vol, err := volume.GetVolume()
		if err != nil {
			println("volume: ", err)
			continue
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

func wifi_update() {
	for true {
		cmd := exec.Command("nmcli", "-f", "NAME,TYPE", "connection", "show", "--active")
		output, err := cmd.Output()
		if err != nil {
			log.Fatal("wifi: ", err)
		}
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		wifi_lock.Lock()
		wifi_data.active = false
		wifi_lock.Unlock()
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.Fields(line)
			if parts[len(parts) - 1] == "wifi" {
				wifi_lock.Lock()
				name := parts[0]
				for i := 1; i < len(parts) - 1; i ++ {
					name = name + " " + parts[i]
				}
				wifi_data.name = name
				wifi_data.active = true
				wifi_lock.Unlock()
				break
			}
		}

		time.Sleep(2000 * time.Millisecond)
	}
}
