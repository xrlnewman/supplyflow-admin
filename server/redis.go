package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisIdempotency struct{ client *redis.Client }

func NewRedisFromEnv(ctx context.Context) (idempotencyStore, func() error, error) {
	addr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	if addr == "" {
		return newMemoryIdempotency(), func() error { return nil }, nil
	}
	client := redis.NewClient(&redis.Options{Addr: addr, Password: os.Getenv("REDIS_PASSWORD"), DB: 0})
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, nil, err
	}
	return &redisIdempotency{client: client}, client.Close, nil
}

func (r *redisIdempotency) Get(ctx context.Context, key string) (string, bool, error) {
	v, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	return v, err == nil, err
}
func (r *redisIdempotency) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}
func (r *redisIdempotency) Lock(ctx context.Context, key string, ttl time.Duration) (func(), error) {
	lockKey := "lock:" + key
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	ok, err := r.client.SetNX(ctx, lockKey, token, ttl).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrIdempotencyBusy
	}
	release := func() {
		_ = r.client.Eval(context.Background(), `if redis.call('get',KEYS[1])==ARGV[1] then return redis.call('del',KEYS[1]) else return 0 end`, []string{lockKey}, token).Err()
	}
	return release, nil
}
