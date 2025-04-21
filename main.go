package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
	"github.com/gorilla/websocket"
)

// request payload
type registerRequest struct {
    EphemeralContactKeysList []string `json:"ephemeralContactKeysList"`
    TempEphemeralUserId      string   `json:"tempEphemeralUserId"`
    HourId                   int64    `json:"hourId"`
}

func main() {
    // if we're on a nuke server, start the half‑past‑hour ticker

	redisCleintCache := &RedisClientConnectionCache{}

    if os.Getenv("IS_NUKE_SERVER") == "true" {
        go ScheduleHalfPastHour(nukeRoutine, redisCleintCache)
    }

    http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }

        var req registerRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
            return
        }

        if err := ValidateHourId(req.HourId); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        rdb, err := GetRedisClient(req.HourId, redisCleintCache)
        if err != nil {
            http.Error(w, "failed to get redis client: "+err.Error(), http.StatusInternalServerError)
            return
        }

        if err := RegisterUser(req.EphemeralContactKeysList, req.TempEphemeralUserId, rdb); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        w.WriteHeader(http.StatusAccepted)
    })

    log.Println("listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

// nukeRoutine holds the work you want done every half past the hour.
func nukeRoutine(cache *RedisClientConnectionCache) {
    log.Println("🚀 running nuke routine at", time.Now().Format(time.RFC3339))
	NukeRedis(GetCurrentHour(), cache)
}
