package request

import (
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("github.com/kujilabo/bamboo/bamboo-lib/request")
