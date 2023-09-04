package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	bamboolib "github.com/kujilabo/bamboo/bamboo-lib"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
	// bamboolib "github.com/kujilabo/bamboo/bamboo-lib"
	// liberrors "github.com/kujilabo/bamboo/lib/errors"
)

func main() {
	fmt.Println("bamboo-app1f")

	body := map[string]interface{}{
		"value": 60,
	}
	bodyJson, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	req := bamboolib.ApplicationRequest{
		DestinationTopic: "topic-x",
		Body:             string(bodyJson),
		RetryIntervalSec: 60,
		MaxRetry:         2,
	}
	fmt.Println(req)
}

func run() error {
	// make a writer that produces to topic-A, using the least-bytes distribution
	w := &kafka.Writer{
		Addr:     kafka.TCP("localhost:29092"),
		Topic:    "my-topic",
		Balancer: &kafka.LeastBytes{},
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"value": 60,
	}
	bodyJson, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req := bamboolib.ApplicationRequest{
		DestinationTopic: "topic-x",
		Body:             string(bodyJson),
		RetryIntervalSec: 60,
		MaxRetry:         2,
	}
	// req := map[string]interface{}{
	// 	"a": string(bodyJson),
	// }
	bytes, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if err := w.WriteMessages(context.Background(),
		kafka.Message{
			Key:   []byte(id.String()),
			Value: bytes,
		},
	); err != nil {
		return liberrors.Errorf("failed to write. err: %w", err)
		// return err
	}

	if err := w.Close(); err != nil {
		return liberrors.Errorf("failed to close writer. err: %w", err)
		// return err
	}

	return nil
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
