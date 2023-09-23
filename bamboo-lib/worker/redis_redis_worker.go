package worker

import (
	"context"
	"encoding/base64"
	"errors"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/protobuf/proto"

	pb "github.com/kujilabo/bamboo/bamboo-lib/proto"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
	"github.com/kujilabo/bamboo/lib/log"
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
	logger := log.FromContext(ctx)
	operation := func() error {
		if err := w.ping(ctx); err != nil {
			return liberrors.Errorf("ping. err: %w", err)
		}

		dispatcher := libworker.NewDispatcher()
		defer dispatcher.Stop(ctx)
		dispatcher.Start(ctx, w.numWorkers)

		consumer := redis.NewUniversalClient(&w.publisherOptions)
		defer consumer.Close()

		logger.Info("START")
		for {
			m, err := consumer.BRPop(ctx, 0, w.consumerChannel).Result()
			if err != nil {
				return err
			}

			if len(m) != 2 {
				return errors.New("Conn error")
			}

			reqStr := m[1]
			reqBytes, err := base64.StdEncoding.DecodeString(reqStr)
			if err != nil {
				logger.Warnf("invalid parameter. failed to base64.StdEncoding.DecodeString. err: %w", err)
				continue
			}

			req := pb.WorkerParameter{}
			if err := proto.Unmarshal(reqBytes, &req); err != nil {
				logger.Warnf("invalid parameter. failed to proto.Unmarshal. err: %w", err)
				continue
			}

			var headers propagation.MapCarrier = req.Headers

			dispatcher.AddJob(NewRedisJob(ctx, headers, req.RequestId, w.publisherOptions, w.workerFn, req.Data, req.ResultChannel))
		}
	}

	backOff := backoff.NewExponentialBackOff()
	backOff.MaxElapsedTime = 0

	notify := func(err error, d time.Duration) {
		logger.Errorf("notify %+v", err)
	}

	err := backoff.RetryNotify(operation, backOff, notify)
	if err != nil {
		return err
	}
	logger.Info("END")
	return nil
}
