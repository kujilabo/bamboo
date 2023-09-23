package worker

import (
	"context"
	"errors"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/protobuf/proto"

	pb "github.com/kujilabo/bamboo/bamboo-lib/proto"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
	"github.com/kujilabo/bamboo/lib/log"
	libworker "github.com/kujilabo/bamboo/lib/worker"
)

type kafkaRedisBambooWorker struct {
	consumerOptions  kafka.ReaderConfig
	publisherOptions redis.UniversalOptions
	workerFn         WorkerFn
	numWorkers       int
	propagator       propagation.TextMapPropagator
}

func NewKafkaRedisBambooWorker(consumerOptions kafka.ReaderConfig, publisherOptions redis.UniversalOptions, workerFn WorkerFn, numWorkers int) BambooWorker {
	return &kafkaRedisBambooWorker{
		consumerOptions:  consumerOptions,
		publisherOptions: publisherOptions,
		workerFn:         workerFn,
		numWorkers:       numWorkers,
		propagator:       otel.GetTextMapPropagator(),
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
	if _, err := publisher.Ping(ctx).Result(); err != nil {
		liberrors.Errorf("publisher.Ping. err: %w", err)
	}

	return nil
}

func (w *kafkaRedisBambooWorker) Run(ctx context.Context) error {
	logger := log.FromContext(ctx)
	operation := func() error {
		if err := w.ping(ctx); err != nil {
			return err
		}

		dispatcher := libworker.NewDispatcher()
		defer dispatcher.Stop(ctx)
		dispatcher.Start(ctx, w.numWorkers)

		r := kafka.NewReader(w.consumerOptions)
		defer r.Close()

		logger.Info("START")
		for {
			m, err := r.ReadMessage(ctx)
			if err != nil {
				return err
			}

			if len(m.Key) == 0 && len(m.Value) == 0 {
				return errors.New("Conn error")
			}

			// fmt.Printf("message at offset %d: %s = %s\n", m.Offset, string(m.Key), string(m.Value))

			req := pb.WorkerParameter{}
			if err := proto.Unmarshal(m.Value, &req); err != nil {
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
