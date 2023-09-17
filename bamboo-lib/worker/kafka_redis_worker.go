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
)

type kafkaRedisBambooWorker struct {
	kafkaReaderConfig kafka.ReaderConfig
	redisOptions      redis.UniversalOptions
	workerFn          WorkerFn
}

func NewKafkaRedisBambooWorker(kafkaReaderConfig kafka.ReaderConfig, redisOptions redis.UniversalOptions, workerFn WorkerFn) BambooWorker {
	return &kafkaRedisBambooWorker{
		kafkaReaderConfig: kafkaReaderConfig,
		redisOptions:      redisOptions,
		workerFn:          workerFn,
	}
}

func (w *kafkaRedisBambooWorker) Run(ctx context.Context) error {
	// for {
	// 	m, err := r.ReadMessage(ctx)
	// 	if err != nil {
	// 		if err := r.Close(); err != nil {
	// 			log.Fatal("failed to close reader:", err)
	// 			return err
	// 		}
	// 	}

	// 	fmt.Printf("message at offset %d: %s = %s\n", m.Offset, string(m.Key), string(m.Value))

	// 	req := bamboorequest.ApplicationRequest{}
	// 	if err := json.Unmarshal(m.Value, &req); err != nil {
	// 		return err
	// 	}

	// 	resData, err := w.workerFn(ctx, req.Data)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	fmt.Println("xxx")

	// 	rdb := redis.NewUniversalClient(&w.redisOptions)
	// 	defer rdb.Close()

	// 	result := rdb.Publish(ctx, req.ResultChannel, resData)
	// 	if result.Err() != nil {
	// 		return result.Err()
	// 	}
	// }

	operation := func() error {
		if err := func() error {
			if len(w.kafkaReaderConfig.Brokers) == 0 {
				return errors.New("broker size is 0")
			}

			conn, err := kafka.Dial("tcp", w.kafkaReaderConfig.Brokers[0])
			if err != nil {
				return liberrors.Errorf("kafka.Dial. err: %w", err)
			}
			defer conn.Close()

			if _, err := conn.ReadPartitions(); err != nil {
				return liberrors.Errorf("conn.ReadPartitions. err: %w", err)
			}
			return nil
		}(); err != nil {
			return err
		}

		if err := func() error {
			rdb := redis.NewUniversalClient(&w.redisOptions)
			defer rdb.Close()
			if result := rdb.Ping(ctx); result.Err() != nil {
				return result.Err()
			}
			return nil
		}(); err != nil {
			return err
		}

		r := kafka.NewReader(w.kafkaReaderConfig)
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

			resData, err := w.workerFn(ctx, req.Data)
			if err != nil {
				return err
			}

			fmt.Println("xxx")

			rdb := redis.NewUniversalClient(&w.redisOptions)
			defer rdb.Close()

			result := rdb.Publish(ctx, req.ResultChannel, resData)
			if result.Err() != nil {
				return result.Err()
			}

			return errors.New("aaa")
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
