package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrCacheMiss = errors.New("cache: key not found")
)

type Cache interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	GetStruct(ctx context.Context, key string, dest any) error
	Delete(ctx context.Context, keys ...string) error
	DeleteByPattern(ctx context.Context, pattern string) error
	Client() redis.UniversalClient
	Publish(ctx context.Context, channel string, message any) error
	Subscribe(ctx context.Context, channel string) *redis.PubSub
	RPush(ctx context.Context, key string, values ...any) error
	BLPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error)
	Allow(ctx context.Context, key string, limit int, rate int) (bool, error)
	SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
}

type redisCache struct {
	client redis.UniversalClient
}

func NewRedisCache(addr, password string, db int) (Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &redisCache{client: client}, nil
}

func (c *redisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	switch v := value.(type) {
	case string, int, int64, float64, bool, []byte:
		err := c.client.Set(ctx, key, v, ttl).Err()
		return err
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return c.client.Set(ctx, key, b, ttl).Err()
	}
}

func (c *redisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

func (c *redisCache) GetStruct(ctx context.Context, key string, dest any) error {
	val, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

func (c *redisCache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

func (c *redisCache) DeleteByPattern(ctx context.Context, pattern string) error {
	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= 100 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
			keys = keys[:0]
		}
	}

	if err := iter.Err(); err != nil {
		return err
	}

	if len(keys) > 0 {
		return c.client.Del(ctx, keys...).Err()
	}
	return nil
}

func (c *redisCache) Client() redis.UniversalClient {
	return c.client
}

func (c *redisCache) Publish(ctx context.Context, channel string, message any) error {
	var payload []byte
	var err error

	switch v := message.(type) {
	case string:
		payload = []byte(v)
	case []byte:
		payload = v
	default:
		payload, err = json.Marshal(v)
		if err != nil {
			return err
		}
	}

	return c.client.Publish(ctx, channel, payload).Err()
}

func (c *redisCache) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	return c.client.Subscribe(ctx, channel)
}

func (c *redisCache) RPush(ctx context.Context, key string, values ...any) error {
	return c.client.RPush(ctx, key, values...).Err()
}

func (c *redisCache) BLPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error) {
	return c.client.BLPop(ctx, timeout, keys...).Result()
}

const rateLimitScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4] or 1)

local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

if tokens == nil then
    tokens = capacity
    last_refill = now
else
    local elapsed = math.max(0, now - last_refill)
    tokens = math.min(capacity, tokens + (elapsed * rate))
    last_refill = now
end

local allowed = 0
if tokens >= requested then
    tokens = tokens - requested
    allowed = 1
end

redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
redis.call('EXPIRE', key, 60)

return { allowed, tokens }
`

func (c *redisCache) Allow(ctx context.Context, key string, limit int, rate int) (bool, error) {
	now := time.Now().Unix()
	res, err := c.client.Eval(ctx, rateLimitScript, []string{key}, limit, rate, now, 1).Result()
	if err != nil {
		return false, err
	}

	results := res.([]interface{})
	allowed := results[0].(int64) == 1
	return allowed, nil
}

func (c *redisCache) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	switch v := value.(type) {
	case string:
		return c.client.SetNX(ctx, key, v, ttl).Result()
	case int, int64, float64, bool:
		return c.client.SetNX(ctx, key, fmt.Sprintf("%v", v), ttl).Result()
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return false, err
		}
		return c.client.SetNX(ctx, key, string(b), ttl).Result()
	}
}
