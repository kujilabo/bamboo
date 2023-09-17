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
	fmt.Println(result.Value)
	return 10, nil
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

	for i := 0; i < 1; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, err := app.call(ctx, "worker1", &worker1Parameter{
				x: 3,
				y: 5,
			}, time.Second*10)
			if err != nil {
				panic(err)
			}
			fmt.Println(val)
		}()
	}
	wg.Wait()

	// t := time.NewTicker(3 * time.Second) // 3秒おきに通知
	// for {
	// 	select {
	// 	case <-t.C:
	// 		// 3秒経過した。ここで何かを行う。
	// 		if err := run(); err != nil {
	// 			t.Stop() // タイマを止める。
	// 			panic(err)
	// 		}
	// 	}
	// }
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
