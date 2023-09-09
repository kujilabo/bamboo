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

func (p *kafkaBambooRequestProducer) Send(ctx context.Context, requestID, traceID, receiverID string, data interface{}) error {
	messageID, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	dataJson, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req := ApplicationRequest{
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

func (p *kafkaBambooRequestProducer) Close(ctx context.Context) error {
	return p.writer.Close()
}
