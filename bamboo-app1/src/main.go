package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/kujilabo/bamboo/bamboo-app1/src/config"
	bamboolib "github.com/kujilabo/bamboo/bamboo-lib"
	libconfig "github.com/kujilabo/bamboo/lib/config"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
)

type StringResult struct {
	Value string
	Error error
}

func main() {
	ctx := context.Background()
	fmt.Println("bamboo-app1")
	rp := NewKafkaBambooRequestProducer("localhost:29092", "my-topic1")
	defer rp.Close(ctx)
	redisChannel, err := uuid.NewRandom()
	if err != nil {
		panic(err)
	}

	wg := sync.WaitGroup{}

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			rs := NewRedisResultSubscriber(ctx)

			ch := make(chan StringResult)
			go func() {
				result, err := rs.SubscribeString(ctx, redisChannel.String(), time.Second*3)

				ch <- StringResult{Value: result, Error: err}
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

type BambooResultSubscriber interface {
	SubscribeString(ctx context.Context, receiverID string, timeout time.Duration) (string, error)
}

type RedisResultSubscriber struct {
	rdb redis.UniversalClient
}

func NewRedisResultSubscriber(ctx context.Context) BambooResultSubscriber {
	rdb := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{"localhost:6379"},
		Password: "", // no password set
	})
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		panic(err)
	}

	return &RedisResultSubscriber{
		rdb: rdb,
	}
}

func (s *RedisResultSubscriber) SubscribeString(ctx context.Context, receiverID string, timeout time.Duration) (string, error) {
	pubsub := s.rdb.Subscribe(ctx, receiverID)
	defer pubsub.Close()
	c1 := make(chan StringResult, 1)

	go func() {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			c1 <- StringResult{Value: "", Error: err}
			return
		}
		c1 <- StringResult{Value: msg.Payload, Error: nil}
	}()

	select {
	case res := <-c1:
		if res.Error != nil {
			return "", res.Error
		}
		return res.Value, nil
	case <-time.After(timeout):
		return "", errors.New("timeout")
	}
}

type BambooRequestProducer interface {
	Send(ctx context.Context, requestID, traceID, receiverID string, data interface{}) error
	Close(ctx context.Context) error
}

type KafkaBambooProducer struct {
	writer *kafka.Writer
}

func NewKafkaBambooRequestProducer(addr, topic string) BambooRequestProducer {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(addr),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}

	return &KafkaBambooProducer{
		writer: writer,
	}
}

func (p *KafkaBambooProducer) Send(ctx context.Context, requestID, traceID, receiverID string, data interface{}) error {
	messageID, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	dataJson, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req := bamboolib.ApplicationRequest{
		RequestID:  requestID,
		TraceID:    traceID,
		MessageID:  messageID.String(),
		ReceiverID: receiverID,
		Data:       dataJson,
	}
	bytes, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if err := p.writer.WriteMessages(context.Background(),
		kafka.Message{
			Key:   []byte(messageID.String()),
			Value: bytes,
		},
	); err != nil {
		return liberrors.Errorf("failed to write. err: %w", err)
		// return err
	}

	return nil
}

func (p *KafkaBambooProducer) Close(ctx context.Context) error {
	return p.writer.Close()
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
