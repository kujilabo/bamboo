package request

import "context"

type BambooRequestProducer interface {
	Produce(ctx context.Context, traceID, redisChannel string, data []byte) error
	Close(ctx context.Context) error
}
