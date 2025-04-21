package main

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
)

// Register a new user
// First it needs an list of string of ephemeral keys called `ephemeralContactKeysList`
// Second there is the `tempEphemeralUserId` which is a string that the user registered with can be used to do another operations
// Third there is the active redis client

// The algorithm is as follows:
// 1. Check if the `ephemeralContactKeysList` is empty
// 2. If it is empty, return an error
// 2.1 Check if the ephemeralContactKeysList is a 256bit hash key
// 2.2 If it is a 256bit hash key, return an error
// 3. Check if the contact list represent a stringifyied 256bit hash key
// 4. If it is not, return an error
// 5. While checking the keys make sure no more than 256 keys are present
// 6. If there are more than 256 keys or one of the keys is not a valid 256bit hash key, return an error
// 7. If all keys are valid we will start the registration process
// 8. We will iterate over each ephemeralContactKey and use it as a key to store the userId
// 9. If the process is successful we will return no error

const MAX_KEYS = 256

func RegisterUser(ephemeralContactKeysList []string, tempEphemeralUserId string, rdb *redis.Client) error {

	// Validate tempEphemeralUserId
	tempEphemeralUserIdValidation := validateKey(tempEphemeralUserId)
	if tempEphemeralUserIdValidation != nil {
		return fmt.Errorf("tempEphemeralUserId is not a valid 256bit hash key: %v", tempEphemeralUserIdValidation)
	}

	// Check if the ephemeralContactKeysList is empty
	contactKeySize := len(ephemeralContactKeysList)

	if contactKeySize == 0{
		return fmt.Errorf("ephemeralContactKeysList is empty")
	}

	if contactKeySize > MAX_KEYS {
		return fmt.Errorf("ephemeralContactKeysList contains more than 256 keys")
	}

	// Check if the ephemeralContactKeysList is a 256bit hash key
	for _, key := range ephemeralContactKeysList {
		keyValidation := validateKey(key)
		if keyValidation != nil {
			return fmt.Errorf("ephemeralContactKeysList contains an invalid key: %v", keyValidation)
		}
	}

	// Prepare the key-value pairs for MSet
	keyValues := make([]string, 0, contactKeySize*2)
	for _, key := range ephemeralContactKeysList {
		keyValues = append(keyValues, key, tempEphemeralUserId)
	}

	// Use MSet to store all keys with the same userId
	ctx := context.Background()
	if err := rdb.MSet(ctx, keyValues).Err(); err != nil {
		return fmt.Errorf("failed to register user: %v", err)
	}


	return nil
}

func validateKey(key string) error {
	// Check if the key is a 256bit hash key
	if len(key) != 64 {
		return fmt.Errorf("key is not a 256bit hash key")
	}
	// Check if the key is a lowercase hex string
	for _, char := range key {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return fmt.Errorf("key is not a lowercase hex string")
		}
	}
	return nil
}