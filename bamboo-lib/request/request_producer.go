package request

import "context"

type BambooRequestProducer interface {
	Send(ctx context.Context, requestID, traceID, receiverID string, data interface{}) error
	Close(ctx context.Context) error
}
