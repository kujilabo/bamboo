package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"

	bambooworker "github.com/kujilabo/bamboo/bamboo-lib/worker"
	"github.com/kujilabo/bamboo/bamboo-worker1/src/config"
	pb "github.com/kujilabo/bamboo/bamboo-worker1/src/proto"
	libconfig "github.com/kujilabo/bamboo/lib/config"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
	"github.com/kujilabo/bamboo/lib/log"
	"github.com/kujilabo/bamboo/lib/worker"
)

var tracer = otel.Tracer("github.com/kujilabo/bamboo/bamboo-worker1")

func getValue(values ...string) string {
	for _, v := range values {
		if len(v) != 0 {
			return v
		}
	}
	return ""
}

type SleepJob struct {
	wg *sync.WaitGroup
}

func (j *SleepJob) Run() error {
	time.Sleep(time.Second * 5)
	j.wg.Done()
	return nil
}

func main0() {
	ctx := context.Background()
	dispatcher := worker.NewDispatcher()
	dispatcher.Start(ctx, 5)
	wg := sync.WaitGroup{}

	wg.Add(5)
	for i := 0; i < 5; i++ {
		dispatcher.JobQueue <- &SleepJob{&wg}
	}
	wg.Wait()
}

func main() {
	ctx := context.Background()
	mode := flag.String("mode", "", "")
	flag.Parse()
	appMode := getValue(*mode, os.Getenv("APP_MODE"), "debug")
	logrus.Infof("mode: %s", appMode)
	fmt.Println("bamboo-worker1")

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
	}, workerFn, 5)

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
	_, span := tracer.Start(ctx, "worker1.fn")
	defer span.End()

	logger := log.FromContext(ctx)

	req := pb.Worker1Parameter{}
	if err := proto.Unmarshal(reqBytes, &req); err != nil {
		logger.Errorf("proto.Unmarshal %+v", err)
		return nil, errors.New("adddd")
	}

	answer := req.X * req.Y
	logger.Infof("answer: %d", answer)
	resp := pb.Worker1Response{Value: answer}
	respBytes, err := proto.Marshal(&resp)
	if err != nil {
		logger.Errorf("proto.Marshal. err: %w", err)
		return nil, liberrors.Errorf("proto.Marshal. err: %w", err)
	}

	return respBytes, nil
}
