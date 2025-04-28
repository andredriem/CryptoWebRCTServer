// This file contains two functions:
// 1. GetRedisClient that returns a redis client based on the hourId
//    1.1 If the hourId is odd it returns the fist redis client defined in the env REDIS_CLIENT_1
//    1.2 If the hourId is even it returns the second redis client defined in the env REDIS_CLIENT_2

// 2. NukeRedis that deletes the redis client based on the hourId
//    2.1 If the hourId is odd it deletes the fist redis client defined in the env REDIS_CLIENT_1
//    2.2 If the hourId is even it deletes the second redis client defined in the env REDIS_CLIENT_2

package main

import (
	"context"
	"log"
	"os"
	"sync"
	"github.com/go-redis/redis/v8"
)

// RedisClientConnectionCache stores redis clients and URIs
type RedisClientConnectionCache struct {
	RedisClient1    *redis.Client
	RedisClient1URI string
	RedisClient2    *redis.Client
	RedisClient2URI string
	mu              sync.Mutex // To ensure thread-safe initialization
}

// GetRedisClient returns a redis client based on the hourId.
// - If hourId is odd → use RedisClient1
// - If hourId is even → use RedisClient2
func GetRedisClient(hourId int64, cache *RedisClientConnectionCache) (*redis.Client, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if hourId%2 != 0 {
		// Odd → Client1
		if cache.RedisClient1 == nil {
			cache.RedisClient1URI = os.Getenv("REDIS_CLIENT_1")
			opts, err := redis.ParseURL(cache.RedisClient1URI)
			if err != nil {
				log.Println("Invalid Redis URI for client1:", err)
				return nil, err
			}
			cache.RedisClient1 = redis.NewClient(opts)
		}
		return cache.RedisClient1, nil
	} else {
		// Even → Client2
		if cache.RedisClient2 == nil {
			cache.RedisClient2URI = os.Getenv("REDIS_CLIENT_2")
			opts, err := redis.ParseURL(cache.RedisClient2URI)
			if err != nil {
				log.Println("Invalid Redis URI for client2:", err)
				return nil, err
			}
			cache.RedisClient2 = redis.NewClient(opts)
		}
		return cache.RedisClient2, nil
	}
}

// NukeRedis flushes the Redis DB of the selected client (based on hourId)
// Any error during the process will be logged and the program will panic since this is a critical operation
// This function is not thread-safe and should be called with caution
func NukeRedis(hourId int64, cache *RedisClientConnectionCache) {
	client, err := GetRedisClient(hourId, cache)
	if err != nil {
		log.Println("Error getting Redis client, nuke aborted:", err)
		panic(err)
	}

	if err := client.FlushDB(context.Background()).Err(); err != nil {
		log.Println("Error flushing Redis database:", err)
		panic(err)
	}

	log.Println("Redis database flushed successfully")
}
