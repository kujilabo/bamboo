package request

import (
	"context"
	"encoding/base64"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/protobuf/proto"

	pb "github.com/kujilabo/bamboo/bamboo-lib/proto"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
	"github.com/kujilabo/bamboo/lib/log"
)

type redisBambooRequestProducer struct {
	workerName      string
	producerOptions redis.UniversalOptions
	producerChannel string
}

func NewRedisBambooRequestProducer(ctx context.Context, workerName string, producerOptions redis.UniversalOptions, producerChannel string) BambooRequestProducer {
	return &redisBambooRequestProducer{
		workerName:      workerName,
		producerOptions: producerOptions,
		producerChannel: producerChannel,
	}
}

func (p *redisBambooRequestProducer) Produce(ctx context.Context, traceID, resultChannel string, data []byte) error {
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

	logger.Infof("LPUSH %s", p.producerChannel)

	producer := redis.NewUniversalClient(&p.producerOptions)
	defer producer.Close()

	if _, err := producer.LPush(ctx, p.producerChannel, reqStr).Result(); err != nil {
		return liberrors.Errorf("producer.LPush. err: %w", err)
	}

	return nil
}

func (p *redisBambooRequestProducer) Close(ctx context.Context) error {
	return nil
}
