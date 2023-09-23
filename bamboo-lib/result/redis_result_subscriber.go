package result

import (
	"context"
	"encoding/base64"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"

	pb "github.com/kujilabo/bamboo/bamboo-lib/proto"
)

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

func (s *RedisResultSubscriber) Subscribe(ctx context.Context, resultChannel string, timeout time.Duration) ([]byte, error) {
	pubsub := s.rdb.Subscribe(ctx, resultChannel)
	defer pubsub.Close()
	c1 := make(chan ByteArreayResult, 1)

	go func() {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			c1 <- ByteArreayResult{Value: nil, Error: err}
			return
		}
		respBytes, err := base64.StdEncoding.DecodeString(msg.Payload)
		if err != nil {
			c1 <- ByteArreayResult{Value: nil, Error: err}
			return
		}

		resp := pb.WorkerResponse{}
		if err := proto.Unmarshal(respBytes, &resp); err != nil {
			c1 <- ByteArreayResult{Value: nil, Error: err}

			return
		}
		c1 <- ByteArreayResult{Value: resp.Data, Error: nil}
	}()

	select {
	case res := <-c1:
		if res.Error != nil {
			return nil, res.Error
		}
		return res.Value, nil
	case <-time.After(timeout):
		return nil, errors.New("timeout")
	}
}
