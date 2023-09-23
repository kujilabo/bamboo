package worker

import (
	"errors"

	"github.com/redis/go-redis/v9"
)

func CreateBambooWorker(cfg *WorkerConfig, workerFn WorkerFn) (BambooWorker, error) {
	if cfg.Consumer.Type == "redis" && cfg.Publisher.Type == "redis" {
		return NewRedisRedisBambooWorker(redis.UniversalOptions{
			Addrs:    cfg.Consumer.Redis.Addrs,
			Password: cfg.Consumer.Redis.Password,
		}, cfg.Consumer.Redis.Channel, redis.UniversalOptions{
			Addrs:    cfg.Publisher.Redis.Addrs,
			Password: cfg.Publisher.Redis.Password,
		}, workerFn, cfg.NumWorkers), nil
	}

	return nil, errors.New("Invalid")
}
