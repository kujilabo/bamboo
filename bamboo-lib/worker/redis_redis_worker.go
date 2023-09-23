package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/redis/go-redis/v9"

	bamboorequest "github.com/kujilabo/bamboo/bamboo-lib/request"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
	libworker "github.com/kujilabo/bamboo/lib/worker"
)

type redisRedisBambooWorker struct {
	consumerOptions  redis.UniversalOptions
	consumerChannel  string
	publisherOptions redis.UniversalOptions
	workerFn         WorkerFn
	numWorkers       int
}

func NewRedisRedisBambooWorker(consumerOptions redis.UniversalOptions, consumerChannel string, publisherOptions redis.UniversalOptions, workerFn WorkerFn, numWorkers int) BambooWorker {
	return &redisRedisBambooWorker{
		consumerOptions:  consumerOptions,
		consumerChannel:  consumerChannel,
		publisherOptions: publisherOptions,
		workerFn:         workerFn,
		numWorkers:       numWorkers,
	}
}

func (w *redisRedisBambooWorker) ping(ctx context.Context) error {
	consumer := redis.NewUniversalClient(&w.consumerOptions)
	defer consumer.Close()
	if result := consumer.Ping(ctx); result.Err() != nil {
		return result.Err()
	}

	publisher := redis.NewUniversalClient(&w.publisherOptions)
	defer publisher.Close()
	if result := publisher.Ping(ctx); result.Err() != nil {
		return result.Err()
	}

	return nil
}

func (w *redisRedisBambooWorker) Run(ctx context.Context) error {
	operation := func() error {
		if err := w.ping(ctx); err != nil {
			return liberrors.Errorf("ping. err: %w", err)
		}

		dispatcher := libworker.NewDispatcher()
		defer dispatcher.Stop(ctx)
		dispatcher.Start(ctx, w.numWorkers)

		consumer := redis.NewUniversalClient(&w.publisherOptions)
		defer consumer.Close()

		fmt.Println("START")
		for {
			m, err := consumer.BRPop(ctx, 0, w.consumerChannel).Result()
			if err != nil {
				return err
			}

			if len(m) != 2 {
				return errors.New("Conn error")
			}

			req := bamboorequest.ApplicationRequest{}
			if err := json.Unmarshal([]byte(m[1]), &req); err != nil {
				return err
			}

			dispatcher.AddJob(ctx, &redisJob{
				publisherOptions: w.publisherOptions,
				workerFn:         w.workerFn,
				parameter:        req.Data,
				resultChannel:    req.ResultChannel,
			})

			// resData, err := w.workerFn(ctx, req.Data)
			// if err != nil {
			// 	return err
			// }

			// fmt.Println("xxx")

			// publisher := redis.NewUniversalClient(&w.publisherOptions)
			// defer publisher.Close()

			// if _, err := publisher.Publish(ctx, req.ResultChannel, resData).Result(); err != nil {
			// 	return err
			// }
		}
	}

	// backOff := backoff.WithContext(newBackOff(), ctx)
	// backOff := &backoff.ZeroBackOff{}
	backOff := backoff.NewExponentialBackOff()

	notify := func(err error, d time.Duration) {
		// logger.Debug().Msgf("New attempt %d for request: %v", attempts, req.URL)

		// r.listener.Retried(req, attempts)
		fmt.Println(err)
	}

	err := backoff.RetryNotify(operation, backOff, notify)
	if err != nil {
		// logger.Debug().Err(err).Msg("Final retry attempt failed")
		return err
	}
	fmt.Println("END")
	return nil
}

// func newBackOff() backoff.BackOff {
// 	// if r.attempts < 2 || r.initialInterval <= 0 {
// 	// 	return &backoff.ZeroBackOff{}
// 	// }

// 	b := backoff.NewExponentialBackOff()
// 	b.InitialInterval = time.Second * 5

// 	// calculate the multiplier for the given number of attempts
// 	// so that applying the multiplier for the given number of attempts will not exceed 2 times the initial interval
// 	// it allows to control the progression along the attempts
// 	b.Multiplier = math.Pow(2, 1/float64(r.attempts-1))

// 	// according to docs, b.Reset() must be called before using
// 	b.Reset()
// 	return b
// }
