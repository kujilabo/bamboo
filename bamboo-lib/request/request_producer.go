package request

import "context"

type BambooRequestProducer interface {
	Produce(ctx context.Context, resultChannel string, data []byte) error
	Close(ctx context.Context) error
}
