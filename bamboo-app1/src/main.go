package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/kujilabo/bamboo/bamboo-app1/src/config"
	bamboorequest "github.com/kujilabo/bamboo/bamboo-lib/request"
	bambooresult "github.com/kujilabo/bamboo/bamboo-lib/result"
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

type app struct {
	rs    bambooresult.BambooResultSubscriber
	rpMap map[string]bamboorequest.BambooRequestProducer
}

func (a *app) newRedisChannelString() (string, error) {
	redisChannel, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	return redisChannel.String(), nil
}

type BambooPrameter interface {
	ToBytes() ([]byte, error)
}

type worker1Parameter struct {
	x int
	y int
}

func (p *worker1Parameter) ToBytes() ([]byte, error) {
	dataBytes, err := json.Marshal(map[string]int{"x": p.x, "y": p.y})
	if err != nil {
		return nil, err
	}
	return dataBytes, nil

}

func (a *app) call(ctx context.Context, callee string, param BambooPrameter, timeout time.Duration) (int, error) {
	redisChannel, err := a.newRedisChannelString()
	if err != nil {
		return 0, err
	}

	rp, ok := a.rpMap[callee]
	if !ok {
		return 0, errors.New("NotFound")
	}

	ch := make(chan bambooresult.StringResult)
	go func() {
		result, err := a.rs.SubscribeString(ctx, redisChannel, timeout)

		ch <- bambooresult.StringResult{Value: result, Error: err}
	}()

	dataBytes, err := param.ToBytes()
	if err != nil {
		return 0, err
	}

	rp.Send(ctx, "def", redisChannel, dataBytes)

	result := <-ch
	if result.Error != nil {
		return 0, result.Error
	}
	resultMap := map[string]int{}
	if err := json.Unmarshal([]byte(result.Value), &resultMap); err != nil {
		return 0, err
	}
	value, ok := resultMap["value"]
	if !ok {
		return 0, errors.New("ValueNOtFound")
	}
	return value, nil
}

type expr struct {
	app *app
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
	if err := e.getError(); err != nil {
		return 0
	}

	val, err := e.app.call(ctx, "worker1", &worker1Parameter{
		x: x, y: y,
	}, time.Second*10)
	if err != nil {
		e.setError(err)
		return 0
	}
	return val
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

	rp := bamboorequest.NewKafkaBambooRequestProducer(cfg.RequestProducer.Kafka.Addr, "topic1")
	defer rp.Close(ctx)

	rs := bambooresult.NewRedisResultSubscriber(ctx, &redis.UniversalOptions{
		Addrs:    cfg.ResultSubscriber.Redis.Addrs,
		Password: cfg.ResultSubscriber.Redis.Password,
	})
	if err := rs.Ping(ctx); err != nil {
		panic(err)
	}
	app := app{
		rs: rs,
		rpMap: map[string]bamboorequest.BambooRequestProducer{
			"worker1": rp,
		},
	}

	wg := sync.WaitGroup{}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e := expr{
				app: &app,
			}
			c := e.worker1(ctx, 3, 5)
			d := e.worker1(ctx, c, 3)
			f := e.worker1(ctx, d, 2)

			fmt.Println(f)
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
