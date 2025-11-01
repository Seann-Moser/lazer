package controller

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

//go:embed index.html
var index []byte

type DaysOfWeek int

const (
	Monday DaysOfWeek = iota
	Tuesdazy
	Wednesday
	Thursday
	Friday
	Saturday
	Sunday
)

type GeneralSetting struct {
	Schedule map[DaysOfWeek][]GeneralSchedule `json:"schedule"`
}
type GeneralSchedule struct {
	OnDuration time.Duration `json:"onDuration,omitempty"`
	StartTime  string        `json:"startTime,omitempty"` // hour,minute of the day
	State      State         `json:"state,omitempty"`
}

func (c *Controller) StartServer(ctx context.Context) {
	if c.Configuration.Setting.Schedule == nil {
		c.Configuration.Setting.Schedule = map[DaysOfWeek][]GeneralSchedule{}
	}

	http.HandleFunc("/", serveFrontend)
	http.HandleFunc("/api/get", c.handleGetSettings)
	http.HandleFunc("/api/save", c.handleSaveSettings)

	fmt.Println("Server running on http://0.0.0.0:8080")

	http.ListenAndServe("0.0.0.0:8080", nil)
}

func serveFrontend(w http.ResponseWriter, r *http.Request) {
	w.Write(index)
}

func (c *Controller) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(c.Configuration.Setting)
}

func (c *Controller) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var newSetting GeneralSetting
	if err := json.NewDecoder(r.Body).Decode(&newSetting); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	c.Configuration.Setting = newSetting
	data, err := json.Marshal(c.Configuration)
	if err != nil {
		log.Printf("failed marshalling config file")
	}
	err = os.WriteFile(configFile, data, 0777)
	if err != nil {
		log.Printf("failed saving config file")
	}

	w.WriteHeader(http.StatusOK)
}
