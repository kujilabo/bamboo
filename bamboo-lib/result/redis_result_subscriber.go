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

type RedisResultSubscriber struct {
	rdb redis.UniversalClient
}

func NewRedisResultSubscriber(ctx context.Context) BambooResultSubscriber {
	rdb := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{"localhost:6379"},
		Password: "", // no password set
	})
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		panic(err)
	}

	return &RedisResultSubscriber{
		rdb: rdb,
	}
}

func (s *RedisResultSubscriber) SubscribeString(ctx context.Context, receiverID string, timeout time.Duration) (string, error) {
	pubsub := s.rdb.Subscribe(ctx, receiverID)
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
