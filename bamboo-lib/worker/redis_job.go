package worker

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type redisJob struct {
	publisherOptions redis.UniversalOptions
	workerFn         WorkerFn
	parameter        []byte
	resultChannel    string
}

func (j *redisJob) Run(ctx context.Context) error {
	result, err := j.workerFn(ctx, j.parameter)
	if err != nil {
		return err
	}

	fmt.Println("xxx")

	publisher := redis.NewUniversalClient(&j.publisherOptions)
	defer publisher.Close()

	if _, err := publisher.Publish(ctx, j.resultChannel, result).Result(); err != nil {
		return err
	}

	return nil
}
