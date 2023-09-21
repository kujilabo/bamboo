package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/sync/errgroup"

	bambooworker "github.com/kujilabo/bamboo/bamboo-lib/worker"
	"github.com/kujilabo/bamboo/bamboo-worker1/src/config"
	libconfig "github.com/kujilabo/bamboo/lib/config"
)

func getValue(values ...string) string {
	for _, v := range values {
		if len(v) != 0 {
			return v
		}
	}
	return ""
}

func main() {
	ctx := context.Background()
	mode := flag.String("mode", "", "")
	flag.Parse()
	appMode := getValue(*mode, os.Getenv("APP_MODE"), "debug")
	logrus.Infof("mode: %s", appMode)
	fmt.Println("bamboo-worker-redis-redis")

	cfg, tp := initialize(ctx, appMode)
	defer tp.ForceFlush(ctx) // flushes any pending spans

	logrus.Infof("config: %+v", cfg.Worker.Kafka)

	gracefulShutdownTime2 := time.Duration(cfg.Shutdown.TimeSec2) * time.Second

	worker := bambooworker.NewKafkaRedisBambooWorker(kafka.ReaderConfig{
		Brokers:  cfg.Worker.Kafka.Brokers,
		GroupID:  cfg.Worker.Kafka.GroupID,
		Topic:    cfg.Worker.Kafka.Topic,
		MaxBytes: 10e6, // 10MB
	}, redis.UniversalOptions{
		Addrs:    cfg.Worker.Redis.Addrs,
		Password: cfg.Worker.Redis.Password,
	}, workerFn)

	result := run(ctx, cfg, worker)

	time.Sleep(gracefulShutdownTime2)
	logrus.Info("exited")
	os.Exit(result)
}

func run(ctx context.Context, cfg *config.Config, worker bambooworker.BambooWorker) int {
	var eg *errgroup.Group
	eg, ctx = errgroup.WithContext(ctx)

	eg.Go(func() error {
		return worker.Run(ctx)
	})
	eg.Go(func() error {
		<-ctx.Done()
		return ctx.Err() // nolint:wrapcheck
	})

	if err := eg.Wait(); err != nil {
		logrus.Error(err)
		return 1
	}
	return 0
}

func initialize(ctx context.Context, mode string) (*config.Config, *sdktrace.TracerProvider) {
	cfg, err := config.LoadConfig(mode)
	if err != nil {
		panic(err)
	}

	// init log
	if err := libconfig.InitLog(mode, cfg.Log); err != nil {
		panic(err)
	}

	// init tracer
	tp, err := libconfig.InitTracerProvider(cfg.App.Name, cfg.Trace)
	if err != nil {
		panic(err)
	}
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return cfg, tp
}

func workerFn(ctx context.Context, reqBytes []byte) ([]byte, error) {
	data := map[string]int{}
	if err := json.Unmarshal([]byte(reqBytes), &data); err != nil {
		return nil, errors.New("adddd")
	}

	answer := data["x"] * data["y"]

	res := map[string]int{"value": answer}
	resJson, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}

	return []byte(resJson), nil
}

// func requestReader(ctx context.Context, kafkaReaderConfig kafka.ReaderConfig, fn bambooworker.WorkerFn) error {
// 	r := kafka.NewReader(kafkaReaderConfig)

// 	for {
// 		m, err := r.ReadMessage(ctx)
// 		if err != nil {
// 			if err := r.Close(); err != nil {
// 				log.Fatal("failed to close reader:", err)
// 				return err
// 			}
// 		}
// 		fmt.Printf("message at offset %d: %s = %s\n", m.Offset, string(m.Key), string(m.Value))

// 		req := bamboorequest.ApplicationRequest{}
// 		if err := json.Unmarshal(m.Value, &req); err != nil {
// 			return err
// 		}

// 		resData, err := fn(ctx, req.Data)
// 		if err != nil {
// 			return err
// 		}

// 		fmt.Println("xxx")

// 		rdb := redis.NewUniversalClient(&redis.UniversalOptions{
// 			Addrs:    []string{"localhost:6379"},
// 			Password: "", // no password set
// 		})
// 		defer rdb.Close()

// 		result := rdb.Publish(ctx, req.ReceiverID, resData)
// 		if result.Err() != nil {
// 			return result.Err()
// 		}
// 	}
// }
