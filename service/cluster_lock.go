package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const clusterLockPrefix = "ai_getaway:cluster_lock:"

func RunWithClusterLock(redisClient *redis.Client, enabled bool, lockName string, owner string, ttl time.Duration, fn func()) {
	if !enabled {
		fn()
		return
	}
	if redisClient == nil {
		log.Printf("cluster job %s skipped: redis client unavailable", lockName)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	token := owner + ":" + randomLockToken()
	key := clusterLockPrefix + lockName
	acquired, err := redisClient.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		log.Printf("cluster job %s skipped: acquire lock failed: %v", lockName, err)
		return
	}
	if !acquired {
		return
	}
	defer releaseClusterLock(redisClient, key, token)

	fn()
}

func releaseClusterLock(redisClient *redis.Client, key string, token string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	const script = `
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
end
return 0
`
	if err := redisClient.Eval(ctx, script, []string{key}, token).Err(); err != nil {
		log.Printf("release cluster lock %s failed: %v", key, err)
	}
}

func randomLockToken() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(buf[:])
}
