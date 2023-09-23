package client

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/kujilabo/bamboo/bamboo-lib/request"
	"github.com/kujilabo/bamboo/bamboo-lib/result"
)

func CreateWorkerClient(ctx context.Context, workerName string, cfg *WorkerClientConfig) WorkerClient {
	var rp request.BambooRequestProducer
	var rs result.BambooResultSubscriber

	if cfg.RequestProducer.Type == "kafka" {
		rp = request.NewKafkaBambooRequestProducer(ctx, workerName, cfg.RequestProducer.Kafka.Addr, cfg.RequestProducer.Kafka.Topic)
	} else if cfg.RequestProducer.Type == "redis" {
		fmt.Println("redis")
		rp = request.NewRedisBambooRequestProducer(ctx, workerName, redis.UniversalOptions{
			Addrs: cfg.RequestProducer.Redis.Addrs,
		}, cfg.RequestProducer.Redis.Channel)
	}
	if cfg.ResultSubscriber.Type == "redis" {
		rs = result.NewRedisResultSubscriber(ctx, workerName, redis.UniversalOptions{
			Addrs:    cfg.ResultSubscriber.Redis.Addrs,
			Password: cfg.ResultSubscriber.Redis.Password,
		})
	}

	return NewWorkerClient(rp, rs)
}
