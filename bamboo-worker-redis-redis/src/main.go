package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"

	bambooworker "github.com/kujilabo/bamboo/bamboo-lib/worker"
	"github.com/kujilabo/bamboo/bamboo-worker-redis-redis/src/config"
	pb "github.com/kujilabo/bamboo/bamboo-worker-redis-redis/src/proto"
	libconfig "github.com/kujilabo/bamboo/lib/config"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
	"github.com/kujilabo/bamboo/lib/log"
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

	p1 := pb.RedisRedisParameter{X: 5, Y: 12}
	p2 := pb.RedisRedisParameter{}
	out, err := proto.Marshal(&p1)
	encoded := base64.StdEncoding.EncodeToString(out)
	fmt.Println(encoded)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err := proto.Unmarshal(decoded, &p2); err != nil {
		panic(err)
	}
	fmt.Println(p2.X)
	fmt.Println(p2.Y)

	cfg, tp := initialize(ctx, appMode)
	defer tp.ForceFlush(ctx) // flushes any pending spans

	gracefulShutdownTime2 := time.Duration(cfg.Shutdown.TimeSec2) * time.Second

	worker, err := bambooworker.CreateBambooWorker(cfg.Worker, workerFn)
	if err != nil {
		panic(err)
	}

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
	logger := log.FromContext(ctx)

	req := pb.RedisRedisParameter{}
	if err := proto.Unmarshal(reqBytes, &req); err != nil {
		logger.Errorf("proto.Unmarshal %+v", err)
		return nil, errors.New("adddd")
	}

	// data := map[string]int{}
	// if err := json.Unmarshal([]byte(reqBytes), &data); err != nil {
	// 	return nil, errors.New("adddd")
	// }

	answer := req.X * req.Y
	logger.Infof("answer: %d", answer)
	resp := pb.RedisRedisResponse{Value: answer}
	respBytes, err := proto.Marshal(&resp)
	if err != nil {
		return nil, liberrors.Errorf("proto.Marshal. err: %w", err)
	}

	return respBytes, nil
}
