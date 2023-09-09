package worker

import "context"

type BambooWorker interface {
	Run(ctx context.Context) error
}

type WorkerFn func(ctx context.Context, data []byte) ([]byte, error)
