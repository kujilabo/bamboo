package result

import (
	"context"
	"time"
)

type BambooResultSubscriber interface {
	Ping(ctx context.Context) error
	Subscribe(ctx context.Context, resultChannel string, timeout time.Duration) ([]byte, error)
}
