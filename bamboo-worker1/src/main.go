package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/sync/errgroup"

	bamboorequest "github.com/kujilabo/bamboo/bamboo-lib/request"
	"github.com/kujilabo/bamboo/bamboo-request-controller/src/config"
	libconfig "github.com/kujilabo/bamboo/lib/config"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
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
	env := flag.String("env", "", "environment")
	flag.Parse()
	appEnv := getValue(*env, os.Getenv("APP_ENV"), "local")
	logrus.Infof("env: %s", appEnv)
	fmt.Println("bamboo-worker1")
	liberrors.Errorf("aa")

	cfg, tp, rdb := initialize(ctx, appEnv)
	defer rdb.Close()
	defer tp.ForceFlush(ctx) // flushes any pending spans

	gracefulShutdownTime2 := time.Duration(cfg.Shutdown.TimeSec2) * time.Second

	worker := NewBambooKafkaRedisWorker(kafka.ReaderConfig{
		Brokers:  []string{"localhost:29092"},
		GroupID:  "consumer-group-id",
		Topic:    "my-topic1",
		MaxBytes: 10e6, // 10MB
	},
		redis.UniversalOptions{
			Addrs:    []string{"localhost:6379"},
			Password: "", // no password set

		}, workerFn)

	result := run(ctx, cfg, worker)

	time.Sleep(gracefulShutdownTime2)
	logrus.Info("exited")
	os.Exit(result)
}

type BambooWorker interface {
	Run(ctx context.Context) error
}

type bambooWorker struct {
	kafkaReaderConfig kafka.ReaderConfig
	redisOptions      redis.UniversalOptions
	workerFn          WorkerFn
}

func NewBambooKafkaRedisWorker(kafkaReaderConfig kafka.ReaderConfig, redisOptions redis.UniversalOptions, workerFn WorkerFn) BambooWorker {
	return &bambooWorker{
		kafkaReaderConfig: kafkaReaderConfig,
		redisOptions:      redisOptions,
		workerFn:          workerFn,
	}
}

func (w *bambooWorker) Run(ctx context.Context) error {
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

func run(ctx context.Context, cfg *config.Config, worker BambooWorker) int {
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

type WorkerFn func(ctx context.Context, data []byte) ([]byte, error)

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

func requestReader(ctx context.Context, kafkaReaderConfig kafka.ReaderConfig, fn WorkerFn) error {
	r := kafka.NewReader(kafkaReaderConfig)

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

		resData, err := fn(ctx, req.Data)
		if err != nil {
			return err
		}

		fmt.Println("xxx")

		rdb := redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:    []string{"localhost:6379"},
			Password: "", // no password set
		})
		defer rdb.Close()

		result := rdb.Publish(ctx, req.ReceiverID, resData)
		if result.Err() != nil {
			return result.Err()
		}
	}
}

func listTopics() {
	conn, err := kafka.Dial("tcp", "localhost:29092")
	if err != nil {
		panic(err.Error())
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions()
	if err != nil {
		panic(err.Error())
	}

	m := map[string]struct{}{}

	for _, p := range partitions {
		fmt.Println(p)
		m[p.Topic] = struct{}{}
	}
	for k := range m {

		fmt.Println(k)
	}
}

func writer() {
	// make a writer that produces to topic-A, using the least-bytes distribution
	w := &kafka.Writer{
		Addr:     kafka.TCP("localhost:29092"),
		Topic:    "my-topic",
		Balancer: &kafka.LeastBytes{},
	}

	err := w.WriteMessages(context.Background(),
		kafka.Message{
			Key:   []byte("Key-A"),
			Value: []byte("Hello World!"),
		},
		kafka.Message{
			Key:   []byte("Key-B"),
			Value: []byte("One!"),
		},
		kafka.Message{
			Key:   []byte("Key-C"),
			Value: []byte("Two!"),
		},
	)
	if err != nil {
		log.Fatal("failed to write messages:", err)
	}

	if err := w.Close(); err != nil {
		log.Fatal("failed to close writer:", err)
	}
}

func produceMessage() {
	// to produce messages
	topic := "my-topic"
	partition := 1

	conn, err := kafka.DialLeader(context.Background(), "tcp", "localhost:29092", topic, partition)
	if err != nil {
		log.Fatal("failed to dial leader:", err)
	}

	fmt.Println("ddd")

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err = conn.WriteMessages(
		kafka.Message{Value: []byte("one!")},
		kafka.Message{Value: []byte("two!")},
		kafka.Message{Value: []byte("three!")},
	)
	if err != nil {
		log.Fatal("failed to write messages:", err)
	}

	if err := conn.Close(); err != nil {
		log.Fatal("failed to close writer:", err)
	}
}

func consumeMessage() {
	// to consume messages
	topic := "my-topic"
	partition := 0

	conn, err := kafka.DialLeader(context.Background(), "tcp", "localhost:29092", topic, partition)
	if err != nil {
		log.Fatal("failed to dial leader:", err)
	}

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	batch := conn.ReadBatch(10e3, 1e6) // fetch 10KB min, 1MB max

	b := make([]byte, 10e3) // 10KB max per message
	for {
		n, err := batch.Read(b)
		if err != nil {
			break
		}
		fmt.Println(string(b[:n]))
	}

	if err := batch.Close(); err != nil {
		log.Fatal("failed to close batch:", err)
	}

	if err := conn.Close(); err != nil {
		log.Fatal("failed to close connection:", err)
	}
}

func reader() {
	// make a new reader that consumes from topic-A, partition 0, at offset 42
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{"localhost:29092"},
		GroupID:  "consumer-group-id",
		Topic:    "my-topic",
		MaxBytes: 10e6, // 10MB
	})
	// r.SetOffset(42)

	for {
		m, err := r.ReadMessage(context.Background())
		if err != nil {
			break
		}
		fmt.Printf("message at offset %d: %s = %s\n", m.Offset, string(m.Key), string(m.Value))
	}

	if err := r.Close(); err != nil {
		log.Fatal("failed to close reader:", err)
	}
}
