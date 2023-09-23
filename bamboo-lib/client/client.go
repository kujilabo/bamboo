package client

import (
	"context"
	"encoding/base64"
	"errors"
	"time"

	"github.com/google/uuid"
	bamboorequest "github.com/kujilabo/bamboo/bamboo-lib/request"
	bambooresult "github.com/kujilabo/bamboo/bamboo-lib/result"
)

type BambooPrameter interface {
	ToBytes() ([]byte, error)
}

type WorkerClient interface {
	Produce(ctx context.Context, traceID, redisChannel string, data []byte) error
	SubscribeString(ctx context.Context, redisChannel string, timeout time.Duration) (string, error)
	Ping(ctx context.Context) error
	Close(ctx context.Context)
}

type workerClient struct {
	rp bamboorequest.BambooRequestProducer
	rs bambooresult.BambooResultSubscriber
}

func NewWorkerClient(rp bamboorequest.BambooRequestProducer, rs bambooresult.BambooResultSubscriber) WorkerClient {
	return &workerClient{
		rp: rp,
		rs: rs,
	}
}

func (c *workerClient) Produce(ctx context.Context, traceID, redisChannel string, data []byte) error {
	return c.rp.Produce(ctx, traceID, redisChannel, data)
}

func (c *workerClient) SubscribeString(ctx context.Context, redisChannel string, timeout time.Duration) (string, error) {
	return c.rs.SubscribeString(ctx, redisChannel, timeout)
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

func (c *StandardClient) Call(ctx context.Context, traceID, clientName string, param []byte, timeout time.Duration) ([]byte, error) {
	redisChannel, err := c.newRedisChannelString()
	if err != nil {
		return nil, err
	}

	client, ok := c.Clients[clientName]
	if !ok {
		return nil, errors.New("NotFound" + clientName)
	}

	ch := make(chan bambooresult.ByteArreayResult)
	go func() {
		result, err := client.SubscribeString(ctx, redisChannel, timeout)
		if err != nil {
			ch <- bambooresult.ByteArreayResult{Value: nil, Error: err}
			return
		}

		decoded, err := base64.StdEncoding.DecodeString(result)
		if err != nil {
			ch <- bambooresult.ByteArreayResult{Value: nil, Error: err}
			return
		}

		ch <- bambooresult.ByteArreayResult{Value: decoded, Error: nil}
	}()

	if err := client.Produce(ctx, traceID, redisChannel, param); err != nil {
		return nil, err
	}

	result := <-ch
	if result.Error != nil {
		return nil, result.Error
	}

	return result.Value, nil
}