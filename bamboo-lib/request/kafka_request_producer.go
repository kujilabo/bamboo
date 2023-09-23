package request

import (
	"context"
	"encoding/base64"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/protobuf/proto"

	pb "github.com/kujilabo/bamboo/bamboo-lib/proto"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
	"github.com/kujilabo/bamboo/lib/log"
)

type kafkaBambooRequestProducer struct {
	workerName string
	writer     *kafka.Writer
}

func NewKafkaBambooRequestProducer(ctx context.Context, workerName, addr, topic string) BambooRequestProducer {
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
	propagator := otel.GetTextMapPropagator()
	headers := propagation.MapCarrier{}

	spanCtx, span := tracer.Start(ctx, p.workerName)
	defer span.End()

	propagator.Inject(spanCtx, headers)
	logger.Infof("carrier: %+v", headers)
	requestID, ok := ctx.Value("request_id").(string)
	if !ok {
		requestID = ""
	}

	messageID, err := uuid.NewRandom()
	if err != nil {
		return liberrors.Errorf("uuid.NewRandom. err: %w", err)
	}

	req := pb.WorkerParameter{
		Headers:       headers,
		RequestId:     requestID,
		ResultChannel: resultChannel,
		Version:       1,
		Data:          data,
	}
	reqBytes, err := proto.Marshal(&req)
	if err != nil {
		return liberrors.Errorf("proto.Marshal. err: %w", err)
	}
	reqStr := base64.StdEncoding.EncodeToString(reqBytes)

	logger.Infof("SEND %s", reqStr)

	msg := kafka.Message{
		Key:   []byte(messageID.String()),
		Value: reqBytes,
	}

	if err := p.writer.WriteMessages(spanCtx, msg); err != nil {
		return liberrors.Errorf("WriteMessages. err: %w", err)
	}

	return nil
}

func (p *kafkaBambooRequestProducer) Close(ctx context.Context) error {
	return p.writer.Close()
}
