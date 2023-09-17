package result

import (
	"context"
	"time"
)

type BambooResultSubscriber interface {
	Ping(ctx context.Context) error
	SubscribeString(ctx context.Context, receiverID string, timeout time.Duration) (string, error)
}
