package result

import (
	"context"
	"time"
)

type BambooResultSubscriber interface {
	SubscribeString(ctx context.Context, receiverID string, timeout time.Duration) (string, error)
}
