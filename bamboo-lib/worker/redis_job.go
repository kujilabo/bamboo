package worker

import (
	"context"
	"encoding/base64"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"

	pb "github.com/kujilabo/bamboo/bamboo-lib/proto"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
	"github.com/kujilabo/bamboo/lib/log"
)

type redisJob struct {
	publisherOptions redis.UniversalOptions
	workerFn         WorkerFn
	TraceID          string
	parameter        []byte
	resultChannel    string
}

func (j *redisJob) Run() error {
	ctx := log.With(context.Background(), log.Str("trace_id", j.TraceID))

	result, err := j.workerFn(ctx, j.parameter)
	if err != nil {
		return err
	}

	publisher := redis.NewUniversalClient(&j.publisherOptions)
	defer publisher.Close()

	resp := pb.WorkerResponse{
		Data: result,
	}
	respBytes, err := proto.Marshal(&resp)
	if err != nil {
		return liberrors.Errorf("proto.Marshal. err: %w", err)
	}
	respStr := base64.StdEncoding.EncodeToString(respBytes)

	if _, err := publisher.Publish(ctx, j.resultChannel, respStr).Result(); err != nil {
		return err
	}

	return nil
}
