package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SystemData holds all temperature data and uses a mutex for safe access.
type SystemData struct {
	mu          sync.Mutex
	currentTemp int
	minTemp     int
	maxTemp     int
	temperatures []int
}

// SystemDataJSON is a struct for JSON serialization.
type SystemDataJSON struct {
	CurrentTemp int `json:"currentTemp"`
	MinTemp     int `json:"minTemp"`
	MaxTemp     int `json:"maxTemp"`
	Temperatures []int `json:"temperatures"`
}

var data = SystemData{
	temperatures: make([]int, 0, 3600), // 60 minuta * 60 sekundi
}

// getGPUTemp runs nvidia-smi and returns the GPU temperature.
func getGPUTemp() (int, error) {
	cmd := exec.Command("nvidia-smi", "--query-gpu=temperature.gpu", "--format=csv,noheader,nounits")
	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return 0, fmt.Errorf("error running nvidia-smi: %w", err)
	}

	tempStr := strings.TrimSpace(out.String())
	temp, err := strconv.Atoi(tempStr)
	if err != nil {
		return 0, fmt.Errorf("error parsing temperature: %w", err)
	}

	return temp, nil
}

// monitorTemperature monitors temperature in the background and updates SystemData.
func monitorTemperature() {
	for {
		temp, err := getGPUTemp()
		if err != nil {
			fmt.Printf("Error reading temperature: %v\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		data.mu.Lock()

		data.currentTemp = temp

		if len(data.temperatures) == 0 {
			data.minTemp = temp
			data.maxTemp = temp
		} else {
			if temp < data.minTemp {
				data.minTemp = temp
			}
			if temp > data.maxTemp {
				data.maxTemp = temp
			}
		}

		data.temperatures = append(data.temperatures, temp)
		if len(data.temperatures) > 3600 {
			data.temperatures = data.temperatures[1:]
		}

		//fmt.Printf("Current GPU temperature: %d°C\n", data.currentTemp)

		data.mu.Unlock()

		time.Sleep(1 * time.Second)
	}
}

// handleData is the HTTP handler for serving temperature data in JSON format.
func handleData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	data.mu.Lock()
	defer data.mu.Unlock()

	output := SystemDataJSON{
		CurrentTemp: data.currentTemp,
		MinTemp: data.minTemp,
		MaxTemp: data.maxTemp,
		Temperatures: data.temperatures,
	}

	err := json.NewEncoder(w).Encode(output)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// startServer starts the local web server.
func startServer() {
	// Serve static files from the 'static' directory
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)
	
	// Handle data endpoint
	http.HandleFunc("/data", handleData)

	fmt.Println("Web server is running on http://localhost:8081")
	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		panic(err)
	}
}

func main() {
	go monitorTemperature()
	time.Sleep(2 * time.Second)
	startServer()
}
