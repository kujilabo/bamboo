package worker

import (
	"context"
	"encoding/base64"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"

	pb "github.com/kujilabo/bamboo/bamboo-lib/proto"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
	"github.com/kujilabo/bamboo/lib/log"
)

type RedisJob interface {
	Run(ctx context.Context) error
}

type redisJob struct {
	carrier          propagation.MapCarrier
	requestID        string
	publisherOptions redis.UniversalOptions
	workerFn         WorkerFn
	parameter        []byte
	resultChannel    string
}

func NewRedisJob(ctx context.Context, carrier propagation.MapCarrier, requestID string, publisherOptions redis.UniversalOptions, workerFn WorkerFn, parameter []byte, resultChannel string) RedisJob {
	return &redisJob{
		carrier:          carrier,
		requestID:        requestID,
		publisherOptions: publisherOptions,
		workerFn:         workerFn,
		parameter:        parameter,
		resultChannel:    resultChannel,
	}
}

func (j *redisJob) Run(ctx context.Context) error {
	propagator := otel.GetTextMapPropagator()
	ctx = propagator.Extract(ctx, j.carrier)

	opts := []oteltrace.SpanStartOption{
		oteltrace.WithAttributes([]attribute.KeyValue{
			{Key: "request_id", Value: attribute.StringValue(j.requestID)},
		}...),
		oteltrace.WithSpanKind(oteltrace.SpanKindConsumer),
	}
	ctx, span := tracer.Start(ctx, "Run", opts...)
	defer span.End()

	ctx = log.With(ctx, log.Str("request_id", j.requestID))

	result, err := j.workerFn(ctx, j.parameter)
	if err != nil {
		return liberrors.Errorf("workerFn. err: %w", err)
	}

	resp := pb.WorkerResponse{Data: result}
	respBytes, err := proto.Marshal(&resp)
	if err != nil {
		return liberrors.Errorf("proto.Marshal. err: %w", err)
	}
	respStr := base64.StdEncoding.EncodeToString(respBytes)

	publisher := redis.NewUniversalClient(&j.publisherOptions)
	defer publisher.Close()
	if _, err := publisher.Publish(ctx, j.resultChannel, respStr).Result(); err != nil {
		return liberrors.Errorf("publisher.Publish. err: %w", err)
	}

	return nil
}
