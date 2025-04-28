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

// --- new WebRTC signaling state:
var (
    upgrader = websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool { return true },
    }
)

// global Redis client cache, accessible to all goroutines
var redisCleintCache = &RedisClientConnectionCache{}

func main() {
    // if we're on a nuke server, start the half‑past‑hour ticker


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

    // WebRTC signaling endpoint
    http.HandleFunc("/ws", wsHandler)

    // serve your static files (index.html + client JS) unmodified
    http.Handle("/", http.FileServer(http.Dir("./static")))

    log.Println("listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

// nukeRoutine holds the work you want done every half past the hour.
func nukeRoutine(cache *RedisClientConnectionCache) {
    log.Println("🚀 running nuke routine at", time.Now().Format(time.RFC3339))
	NukeRedis(GetCurrentHour(), cache)
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println("ws upgrade:", err)
        return
    }
    defer conn.Close()

    privateRoomName := r.URL.Query().Get("room")
    // if neither is set close the connection
    if privateRoomName == ""{
        log.Println("ws no room name")
        return
    }

    // Get the current hour
    currentHour := GetCurrentHour()
    // Get the Redis client based on the current hour
    redisClient, err := GetRedisClient(currentHour, redisCleintCache)
    if err != nil {
        log.Println("ws get redis client:", err)
        return
    }

    HandleRoom(privateRoomName, redisClient, conn)
}
