package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

// Coroutine that listens to the redis channel roomname and parses the messages
// While both room owner and non roon owner are subscribed to the same channel
// The room owner can only receive messages from the non room owners
// Room room owners can only send messages to non room owners trough their own private channel
func parseRoomMessages(
    ctx context.Context,
    roomName string,
    redisClient *redis.Client,
    webSocketConn *websocket.Conn,
    webSocketMutex *sync.Mutex,
) error {
    // 1) Subscribe + ensure cleanup
    pubsub := redisClient.Subscribe(ctx, roomName)
    defer pubsub.Close()

    // 2) Wait for the SUBSCRIBE ack
    if _, err := pubsub.Receive(ctx); err != nil {
        return fmt.Errorf("subscribe %q failed: %w", roomName, err)
    }

    // 3) Get the message channel
    msgCh := pubsub.Channel()

    // 5) Main loop: multiplex between ctx.Done() and incoming messages
    for {
        select {
        case <-ctx.Done():
            log.Println("parseRoomMessages: context canceled, exiting")
            return nil

        case msg, ok := <-msgCh:
            if !ok {
                // Channel closed by pubsub.Close()
                log.Println("parseRoomMessages: redis channel closed")
                return nil
            }
            if msg.Payload == "" {
                // no-op on empty publishes
                continue
            }

            // 6) Write to WebSocket atomically
            webSocketMutex.Lock()
            err := webSocketConn.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
            webSocketMutex.Unlock()

            if err != nil {
                // Writing to a dead WS will err here,
                // so we can cancel and return
                log.Printf("write to websocket failed: %v", err)
                return err
            }
        }
    }
}


// Coroutine tat listens to the current websocket connection for incoming messages and broadcasts it to the
// appropriate redis channel
func parseWebSocketMessages(
	ctx context.Context,
	redisClient *redis.Client,
	webSocketConn *websocket.Conn,
	webSocketMutex *sync.Mutex,
) error {

	for {
		// Check if the context is done
		select {
		case <-ctx.Done():
			log.Println("Context done, stopping websocket message parsing")
			return nil
		default:
			// Continue to listen for messages
		}

		// Wait for messages
		_, msg, err := webSocketConn.ReadMessage()
		if err != nil {
			log.Println("Error reading message from websocket here is the stringified message:", string(msg))
			// Connection is probably closed or ther is an error, either way this 
			// error handling acts as a way to stop all the coroutines when the websocket
			// closes since we are always checkick for new socket messages
			log.Println("Error reading message from websocket:", err)
			return err
		}

		// Parse the message as json
		var m map[string]interface{}
		if err := json.Unmarshal(msg, &m); err != nil {
			log.Println("Error unmarshalling message from websocket:", err)
			continue
		}
		// gets attribute ephemeralId from the message
		ephemeralId, ok := m["ephemeralId"].(string)
		if !ok {
			log.Println("Error getting ephemeralId from message:", m)
			continue
		}
		// gets secret id from ephemeralId by consulting redis with an get
		secretId, err := redisClient.Get(ctx, ephemeralId).Result()
		if err != nil {
			if err == redis.Nil {
				log.Println("EphemeralId not found in Redis:", ephemeralId)
				continue
			}
			log.Println("Error getting secretId from Redis:", err)
			continue
		}

		// Publish the whole raw message to the redis channel that has the secretId
		err = redisClient.Publish(ctx, secretId, string(msg)).Err()
		if err != nil {
			log.Println("Error publishing message to Redis channel:", err)
			continue
		}
	}
}


func HandleRoom(
    roomName string,
    redisClient *redis.Client,
    webSocketConn *websocket.Conn,
) {
    // create cancellable context
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    var wg sync.WaitGroup
    wg.Add(2)

    // mutex for serializing writes to the WebSocket
    webSocketMutex := &sync.Mutex{}

    // Redis → Wait for messages incoming to this current room
	// and re-broadcast them to the websocket connection if needed
    go func() {
        defer wg.Done()
        if err := parseRoomMessages(ctx, roomName, redisClient, webSocketConn, webSocketMutex); err != nil {
            log.Println("parseRoomMessages error:", err)
            cancel()
        }
    }()

    // WebSocket → Reads incoming messages from this websocket connection
	// and publishes them to the Redis channel so other clients can receive them
    go func() {
        defer wg.Done()
        if err := parseWebSocketMessages(ctx, redisClient, webSocketConn, webSocketMutex); err != nil {
            log.Println("parseWebSocketMessages error:", err)
            cancel()
        }
    }()

    // wait for either parser to finish + cancel
    wg.Wait()
}