package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"

	"github.com/kujilabo/bamboo/bamboo-app1/src/config"
	"github.com/kujilabo/bamboo/bamboo-lib/client"
	worker_redis_redis_pb "github.com/kujilabo/bamboo/bamboo-worker-redis-redis/src/proto"
	worker1_pb "github.com/kujilabo/bamboo/bamboo-worker1/src/proto"
	libconfig "github.com/kujilabo/bamboo/lib/config"
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

type expr struct {
	app *client.StandardClient
	err error
	mu  sync.Mutex
}

func (e *expr) getError() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.err != nil {
		return e.err
	}
	return nil
}

func (e *expr) setError(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.err = err
}

func (e *expr) worker1(ctx context.Context, x, y int) int {
	logger := log.FromContext(ctx)
	if err := e.getError(); err != nil {
		logger.Info("%+v", err)
		return 0
	}

	p1 := worker1_pb.Worker1Parameter{X: int32(x), Y: int32(y)}
	paramBytes, err := proto.Marshal(&p1)
	if err != nil {
		logger.Info("%+v", err)

		e.setError(err)
		return 0
	}

	respBytes, err := e.app.Call(ctx, "def", "worker1", paramBytes, time.Second*10)
	if err != nil {
		logger.Info("%+v", err)
		e.setError(err)
		return 0
	}

	resp := worker1_pb.Worker1Response{}
	if err := proto.Unmarshal(respBytes, &resp); err != nil {
		logger.Info("%+v", err)
		e.setError(err)
		return 0
	}

	return int(resp.Value)
}

func (e *expr) workerRedisRedis(ctx context.Context, x, y int) int {
	if err := e.getError(); err != nil {
		return 0
	}

	p1 := worker_redis_redis_pb.RedisRedisParameter{X: int32(x), Y: int32(y)}
	paramBytes, err := proto.Marshal(&p1)
	if err != nil {
		e.setError(err)
		return 0
	}

	respBytes, err := e.app.Call(ctx, "def", "worker-redis-redis", paramBytes, time.Second*10)
	if err != nil {
		e.setError(err)
		return 0
	}

	resp := worker_redis_redis_pb.RedisRedisResponse{}
	if err := proto.Unmarshal(respBytes, &resp); err != nil {
		e.setError(err)
		return 0
	}

	return int(resp.Value)
}

func main() {
	ctx := context.Background()
	mode := flag.String("mode", "", "")
	flag.Parse()
	appMode := getValue(*mode, os.Getenv("APP_MODE"), "debug")
	logrus.Infof("mode: %s", appMode)
	fmt.Println("bamboo-app1")

	cfg, tp := initialize(ctx, appMode)
	defer tp.ForceFlush(ctx) // flushes any pending spans

	clients := map[string]client.WorkerClient{}
	for k, v := range cfg.Workers {
		clients[k] = client.CreateWorkerClient(ctx, v)
		defer clients[k].Close(ctx)
	}

	app := client.StandardClient{Clients: clients}

	wg := sync.WaitGroup{}

	for i := 0; i < 1; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			spanCtx, span := tracer.Start(ctx, "TraceLog")
			defer span.End()
			sc := trace.SpanFromContext(spanCtx).SpanContext()
			if !sc.TraceID().IsValid() || !sc.SpanID().IsValid() {
				return
			}

			logCtx := log.With(ctx, log.Str("trace_id", sc.TraceID().String()))
			logger := log.FromContext(logCtx)
			logger.Info("Start")

			e := expr{
				app: &app,
			}

			c := e.worker1(spanCtx, 3, 5)
			d := e.worker1(spanCtx, c, 3)
			f := e.worker1(spanCtx, d, 2)
			g := e.workerRedisRedis(spanCtx, f, 9)

			fmt.Println(c)
			fmt.Println(d)
			fmt.Println(f)
			fmt.Println(g)
			fmt.Println(e.err)
		}()
	}
	wg.Wait()
}

func initialize(ctx context.Context, env string) (*config.Config, *sdktrace.TracerProvider) {
	cfg, err := config.LoadConfig(env)
	if err != nil {
		panic(err)
	}

	// init log
	if err := libconfig.InitLog(env, cfg.Log); err != nil {
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
