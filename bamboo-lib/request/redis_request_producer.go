package request

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	liberrors "github.com/kujilabo/bamboo/lib/errors"
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

func (p *redisBambooRequestProducer) Send(ctx context.Context, traceID, resultChannel string, data []byte) error {
	producer := redis.NewUniversalClient(&p.producerOptions)
	defer producer.Close()

	requestID, err := uuid.NewRandom()
	if err != nil {
		return liberrors.Errorf("uuid.NewRandom. err: %w", err)
	}

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

	if _, err := producer.LPush(ctx, "", string(reqBytes)).Result(); err != nil {
		return liberrors.Errorf("write. err: %w", err)
	}

	return nil
}

func (p *redisBambooRequestProducer) Close(ctx context.Context) error {
	return nil
}
