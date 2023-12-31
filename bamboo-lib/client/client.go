package client

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/kujilabo/bamboo/bamboo-lib/request"
	"github.com/kujilabo/bamboo/bamboo-lib/result"
)

type BambooPrameter interface {
	ToBytes() ([]byte, error)
}

type WorkerClient interface {
	Produce(ctx context.Context, resultChannel string, data []byte) error
	Subscribe(ctx context.Context, resultChannel string, timeout time.Duration) ([]byte, error)
	Ping(ctx context.Context) error
	Close(ctx context.Context)
}

type workerClient struct {
	rp request.BambooRequestProducer
	rs result.BambooResultSubscriber
}

func NewWorkerClient(rp request.BambooRequestProducer, rs result.BambooResultSubscriber) WorkerClient {
	return &workerClient{
		rp: rp,
		rs: rs,
	}
}

func (c *workerClient) Produce(ctx context.Context, resultChannel string, data []byte) error {
	return c.rp.Produce(ctx, resultChannel, data)
}

func (c *workerClient) Subscribe(ctx context.Context, redisChannel string, timeout time.Duration) ([]byte, error) {
	return c.rs.Subscribe(ctx, redisChannel, timeout)
}

func (c *workerClient) Ping(ctx context.Context) error {
	return c.rs.Ping(ctx)
}

func (c *workerClient) Close(ctx context.Context) {
	defer c.rp.Close(ctx)
}

type StandardClient struct {
	Clients map[string]WorkerClient
}

func (c *StandardClient) newRedisChannelString() (string, error) {
	redisChannel, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	return redisChannel.String(), nil
}

func (c *StandardClient) Call(ctx context.Context, clientName string, param []byte, timeout time.Duration) ([]byte, error) {
	redisChannel, err := c.newRedisChannelString()
	if err != nil {
		return nil, err
	}

	client, ok := c.Clients[clientName]
	if !ok {
		return nil, errors.New("NotFound" + clientName)
	}

	ch := make(chan result.ByteArreayResult)
	go func() {
		resultBytes, err := client.Subscribe(ctx, redisChannel, timeout)
		if err != nil {
			ch <- result.ByteArreayResult{Value: nil, Error: err}
			return
		}

		ch <- result.ByteArreayResult{Value: resultBytes, Error: nil}
	}()

	if err := client.Produce(ctx, redisChannel, param); err != nil {
		return nil, err
	}

	result := <-ch
	if result.Error != nil {
		return nil, result.Error
	}

	return result.Value, nil
}
