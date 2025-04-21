package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
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
    rooms   = make(map[string]map[*websocket.Conn]bool)
    roomsMu sync.Mutex

    upgrader = websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool { return true },
    }
)

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

    var roomName string

    for {
        mt, msg, err := conn.ReadMessage()
        if err != nil {
            // cleanup
            roomsMu.Lock()
            if roomName != "" {
                delete(rooms[roomName], conn)
                if len(rooms[roomName]) == 0 {
                    delete(rooms, roomName)
                }
            }
            roomsMu.Unlock()
            return
        }

        // dispatch join vs relay
        var m map[string]interface{}
        if err := json.Unmarshal(msg, &m); err != nil {
            log.Println("ws json:", err)
            continue
        }

        if m["type"] == "join" {
            if rn, ok := m["room"].(string); ok {
                roomName = rn
                roomsMu.Lock()
                if rooms[rn] == nil {
                    rooms[rn] = make(map[*websocket.Conn]bool)
                }
                rooms[rn][conn] = true
                roomsMu.Unlock()
            }
            continue
        }

        // broadcast to peers
        roomsMu.Lock()
        peers := rooms[roomName]
        roomsMu.Unlock()
        for peer := range peers {
            if peer == conn {
                continue
            }
            if err := peer.WriteMessage(mt, msg); err != nil {
                log.Println("ws write:", err)
            }
        }
    }
}
