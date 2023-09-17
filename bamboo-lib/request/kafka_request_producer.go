package request

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	liberrors "github.com/kujilabo/bamboo/lib/errors"
)

type kafkaBambooRequestProducer struct {
	writer *kafka.Writer
}

func NewKafkaBambooRequestProducer(addr, topic string) BambooRequestProducer {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(addr),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}

	return &kafkaBambooRequestProducer{
		writer: writer,
	}
}

func (p *kafkaBambooRequestProducer) Send(ctx context.Context, traceID, resultChannel string, data []byte) error {
	requestID, err := uuid.NewRandom()
	if err != nil {
		return liberrors.Errorf("uuid.NewRandom. err: %w", err)
	}

	// dataJson, err := json.Marshal(data)
	// if err != nil {
	// 	return err
	// }

	req := ApplicationRequest{
		RequestID:     requestID.String(),
		TraceID:       traceID,
		ResultChannel: resultChannel,
		Data:          data,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return liberrors.Errorf("json.Marshal. err: %w", err)
	}

	if err := p.writer.WriteMessages(context.Background(),
		kafka.Message{
			Key:   []byte(requestID.String()),
			Value: reqBytes,
		},
	); err != nil {
		return liberrors.Errorf("write. err: %w", err)
	}

	return nil
}

func (p *kafkaBambooRequestProducer) Close(ctx context.Context) error {
	return p.writer.Close()
}
