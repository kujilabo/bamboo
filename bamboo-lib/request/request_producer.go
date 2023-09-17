package request

import "context"

type BambooRequestProducer interface {
	Send(ctx context.Context, traceID, subscriberID string, data []byte) error
	Close(ctx context.Context) error
}
