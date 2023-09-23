package request

import (
	"context"
	"encoding/base64"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"

	pb "github.com/kujilabo/bamboo/bamboo-lib/proto"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
	"github.com/kujilabo/bamboo/lib/log"
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

func (p *kafkaBambooRequestProducer) Produce(ctx context.Context, traceID, resultChannel string, data []byte) error {
	logger := log.FromContext(ctx)

	requestID, err := uuid.NewRandom()
	if err != nil {
		return liberrors.Errorf("uuid.NewRandom. err: %w", err)
	}

	// dataJson, err := json.Marshal(data)
	// if err != nil {
	// 	return err
	// }

	// req := ApplicationRequest{
	// 	RequestID:     requestID.String(),
	// 	TraceID:       traceID,
	// 	ResultChannel: resultChannel,
	// 	Data:          data,
	// }
	req := pb.WorkerParameter{
		RequestId:     requestID.String(),
		TraceId:       traceID,
		ResultChannel: resultChannel,
		Data:          data,
	}
	reqBytes, err := proto.Marshal(&req)
	if err != nil {
		return liberrors.Errorf("proto.Marshal. err: %w", err)
	}
	reqStr := base64.StdEncoding.EncodeToString(reqBytes)

	// reqBytes, err := json.Marshal(req)
	// if err != nil {
	// 	return liberrors.Errorf("json.Marshal. err: %w", err)
	// }
	logger.Infof("SEND %s", reqStr)

	if err := p.writer.WriteMessages(context.Background(),
		kafka.Message{
			Key:   []byte(requestID.String()),
			Value: reqBytes,
		},
	); err != nil {
		return liberrors.Errorf("WriteMessages. err: %w", err)
	}

	return nil
}

func (p *kafkaBambooRequestProducer) Close(ctx context.Context) error {
	return p.writer.Close()
}
