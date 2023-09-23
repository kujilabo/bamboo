package client

import (
	"github.com/kujilabo/bamboo/bamboo-lib/request"
	"github.com/kujilabo/bamboo/bamboo-lib/result"
)

type WorkerClientConfig struct {
	RequestProducer  *request.RequestProducerConfig `yaml:"requestProducer" validate:"required"`
	ResultSubscriber *result.ResultSubscriberConfig `yaml:"resultSubscriber" validate:"required"`
}
