package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"

	bamboorequest "github.com/kujilabo/bamboo/bamboo-lib/request"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
	libworker "github.com/kujilabo/bamboo/lib/worker"
)

type kafkaRedisBambooWorker struct {
	consumerOptions  kafka.ReaderConfig
	publisherOptions redis.UniversalOptions
	workerFn         WorkerFn
}

type job struct {
	publisherOptions redis.UniversalOptions
	workerFn         WorkerFn
	parameter        []byte
	resultChannel    string
}

func (j *job) Run(ctx context.Context) error {
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

func NewKafkaRedisBambooWorker(consumerOptions kafka.ReaderConfig, publisherOptions redis.UniversalOptions, workerFn WorkerFn) BambooWorker {

	return &kafkaRedisBambooWorker{
		consumerOptions:  consumerOptions,
		publisherOptions: publisherOptions,
		workerFn:         workerFn,
	}
}

func (w *kafkaRedisBambooWorker) ping(ctx context.Context) error {
	if len(w.consumerOptions.Brokers) == 0 {
		return errors.New("broker size is 0")
	}

	conn, err := kafka.Dial("tcp", w.consumerOptions.Brokers[0])
	if err != nil {
		return liberrors.Errorf("kafka.Dial. err: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ReadPartitions(); err != nil {
		return liberrors.Errorf("conn.ReadPartitions. err: %w", err)
	}

	publisher := redis.NewUniversalClient(&w.publisherOptions)
	defer publisher.Close()
	if result := publisher.Ping(ctx); result.Err() != nil {
		return result.Err()
	}

	return nil
}

func (w *kafkaRedisBambooWorker) Run(ctx context.Context) error {

	operation := func() error {
		if err := w.ping(ctx); err != nil {
			return err
		}

		dispatcher := libworker.NewDispatcher()
		defer dispatcher.Stop(ctx)
		dispatcher.Start(ctx, 5)

		r := kafka.NewReader(w.consumerOptions)
		defer r.Close()

		fmt.Println("START")
		for {
			m, err := r.ReadMessage(ctx)
			if err != nil {
				if err := r.Close(); err != nil {
					log.Fatal("failed to close reader:", err)
					return err
				}
			}

			if len(m.Key) == 0 && len(m.Value) == 0 {
				return errors.New("Conn error")
			}

			fmt.Printf("message at offset %d: %s = %s\n", m.Offset, string(m.Key), string(m.Value))

			req := bamboorequest.ApplicationRequest{}
			if err := json.Unmarshal(m.Value, &req); err != nil {
				return err
			}

			dispatcher.AddJob(ctx, &job{
				publisherOptions: w.publisherOptions,
				workerFn:         w.workerFn,
				parameter:        req.Data,
				resultChannel:    req.ResultChannel,
			})
		}
	}

	// backOff := backoff.WithContext(r.newBackOff(), req.Context())
	backOff := &backoff.ZeroBackOff{}

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
// 	if r.attempts < 2 || r.initialInterval <= 0 {
// 		return &backoff.ZeroBackOff{}
// 	}

// 	b := backoff.NewExponentialBackOff()
// 	b.InitialInterval = r.initialInterval

// 	// calculate the multiplier for the given number of attempts
// 	// so that applying the multiplier for the given number of attempts will not exceed 2 times the initial interval
// 	// it allows to control the progression along the attempts
// 	b.Multiplier = math.Pow(2, 1/float64(r.attempts-1))

// 	// according to docs, b.Reset() must be called before using
// 	b.Reset()
// 	return b
// }
