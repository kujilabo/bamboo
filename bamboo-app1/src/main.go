package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/kujilabo/bamboo/bamboo-app1/src/config"
	bamboorequest "github.com/kujilabo/bamboo/bamboo-lib/request"
	bambooresult "github.com/kujilabo/bamboo/bamboo-lib/result"
	libconfig "github.com/kujilabo/bamboo/lib/config"
)

func main() {
	ctx := context.Background()
	fmt.Println("bamboo-app1")
	rp := bamboorequest.NewKafkaBambooRequestProducer("localhost:29092", "my-topic1")
	defer rp.Close(ctx)
	redisChannel, err := uuid.NewRandom()
	if err != nil {
		panic(err)
	}

	wg := sync.WaitGroup{}

	for i := 0; i < 1; i++ {
		wg.Add(1)
		go func() {
			rs := bambooresult.NewRedisResultSubscriber(ctx)

			ch := make(chan bambooresult.StringResult)
			go func() {
				result, err := rs.SubscribeString(ctx, redisChannel.String(), time.Second*3)

				ch <- bambooresult.StringResult{Value: result, Error: err}
			}()

			rp.Send(ctx, "abc", "def", redisChannel.String(), map[string]int{"x": 3, "y": 5})

			result := <-ch
			if result.Error != nil {
				if result.Error == nil {
					panic(errors.New("NIL"))
				}
				panic(result.Error)
			}
			fmt.Println(result.Value)
			wg.Done()
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

func initialize(ctx context.Context, env string) (*config.Config, *sdktrace.TracerProvider, redis.UniversalClient) {
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

	rdb := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{"localhost:6379"},
		Password: "", // no password set
	})
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		panic(err)
	}

	return cfg, tp, rdb
}
