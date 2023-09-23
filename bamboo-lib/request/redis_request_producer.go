package request

import (
	"context"
	"encoding/base64"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"

	pb "github.com/kujilabo/bamboo/bamboo-lib/proto"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
	"github.com/kujilabo/bamboo/lib/log"
)

type redisBambooRequestProducer struct {
	producerOptions redis.UniversalOptions
	producerChannel string
}

func NewRedisBambooRequestProducer(producerOptions redis.UniversalOptions, producerChannel string) BambooRequestProducer {
	return &redisBambooRequestProducer{
		producerOptions: producerOptions,
		producerChannel: producerChannel,
	}
}

func (p *redisBambooRequestProducer) Produce(ctx context.Context, traceID, resultChannel string, data []byte) error {
	logger := log.FromContext(ctx)

	producer := redis.NewUniversalClient(&p.producerOptions)
	defer producer.Close()

	requestID, err := uuid.NewRandom()
	if err != nil {
		return liberrors.Errorf("uuid.NewRandom. err: %w", err)
	}

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

	logger.Infof("LPUSH %s", p.producerChannel)

	if _, err := producer.LPush(ctx, p.producerChannel, reqStr).Result(); err != nil {
		return liberrors.Errorf("LPush. err: %w", err)
	}

	return nil
}

func (p *redisBambooRequestProducer) Close(ctx context.Context) error {
	return nil
}
