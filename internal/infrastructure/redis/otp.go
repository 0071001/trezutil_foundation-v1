package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisOTPStore implements domain.OTPStore using Redis.
type RedisOTPStore struct {
	client *redis.Client
}

// NewRedisOTPStore creates a new Redis OTP store.
func NewRedisOTPStore(client *redis.Client) *RedisOTPStore {
	return &RedisOTPStore{client: client}
}

// Get retrieves an OTP code from Redis.
func (s *RedisOTPStore) Get(ctx context.Context, key string) (*string, error) {
	val, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get failed: %w", err)
	}
	return &val, nil
}

// Set stores an OTP code in Redis with expiration.
func (s *RedisOTPStore) Set(ctx context.Context, key, value string, expiration int64) error {
	err := s.client.Set(ctx, key, value, time.Duration(expiration)*time.Second).Err()
	if err != nil {
		return fmt.Errorf("redis set failed: %w", err)
	}
	return nil
}

// Delete removes an OTP code from Redis.
func (s *RedisOTPStore) Delete(ctx context.Context, key string) error {
	err := s.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("redis delete failed: %w", err)
	}
	return nil
}

// RedisOTPProvider implements domain.OTPProvider using Redis for dev/test.
// In production, replace with actual SMS provider (Twilio, AWS SNS, Africa's Talking).
type RedisOTPProvider struct {
	// Provider-specific configuration
}

// NewRedisOTPProvider creates a new OTP provider.
func NewRedisOTPProvider() *RedisOTPProvider {
	return &RedisOTPProvider{}
}

// Send sends an OTP code via SMS.
// This is a DEV/TEST implementation. In production, integrate with real SMS provider.
func (p *RedisOTPProvider) Send(ctx context.Context, destination string, code string) error {
	// DEV/TEST: log to console (never log OTP in production!)
	_ = destination
	_ = code
	_ = ctx
	return nil
}
