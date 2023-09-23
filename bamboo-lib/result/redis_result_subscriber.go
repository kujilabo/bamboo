package result

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type StringResult struct {
	Value string
	Error error
}
type ByteArreayResult struct {
	Value []byte
	Error error
}

type RedisResultSubscriber struct {
	rdb redis.UniversalClient
}

func NewRedisResultSubscriber(ctx context.Context, redisConfig redis.UniversalOptions) BambooResultSubscriber {
	rdb := redis.NewUniversalClient(&redisConfig)

	return &RedisResultSubscriber{
		rdb: rdb,
	}
}
func (s *RedisResultSubscriber) Ping(ctx context.Context) error {
	if _, err := s.rdb.Ping(ctx).Result(); err != nil {
		return err
	}

	return nil
}

func (s *RedisResultSubscriber) SubscribeString(ctx context.Context, redisChannel string, timeout time.Duration) (string, error) {
	pubsub := s.rdb.Subscribe(ctx, redisChannel)
	defer pubsub.Close()
	c1 := make(chan StringResult, 1)

	go func() {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			c1 <- StringResult{Value: "", Error: err}
			return
		}
		c1 <- StringResult{Value: msg.Payload, Error: nil}
	}()

	select {
	case res := <-c1:
		if res.Error != nil {
			return "", res.Error
		}
		return res.Value, nil
	case <-time.After(timeout):
		return "", errors.New("timeout")
	}
}
