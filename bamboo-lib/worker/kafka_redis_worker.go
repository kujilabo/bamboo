package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
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
	if err := func() error {
		if len(w.kafkaReaderConfig.Brokers) == 0 {
			return errors.New("broker size is 0")
		}

		conn, err := kafka.Dial("tcp", w.kafkaReaderConfig.Brokers[0])
		if err != nil {
			panic(err.Error())
		}
		defer conn.Close()

		if _, err := conn.ReadPartitions(); err != nil {
			return err
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

	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			if err := r.Close(); err != nil {
				log.Fatal("failed to close reader:", err)
				return err
			}
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

		result := rdb.Publish(ctx, req.ReceiverID, resData)
		if result.Err() != nil {
			return result.Err()
		}
	}
}
