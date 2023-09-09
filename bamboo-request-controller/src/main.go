package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	bamboolib "github.com/kujilabo/bamboo/bamboo-lib"
	"github.com/kujilabo/bamboo/bamboo-request-controller/src/config"
	"github.com/kujilabo/bamboo/bamboo-request-controller/src/sqls"
	libconfig "github.com/kujilabo/bamboo/lib/config"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
)

// storage_sqls "github.com/kujilabo/bamboo/bamboo-storage/sqls"
// libconfig "github.com/kujilabo/bamboo/lib/config"

func getValue(values ...string) string {
	for _, v := range values {
		if len(v) != 0 {
			return v
		}
	}
	return ""
}

func main() {
	ctx := context.Background()
	env := flag.String("env", "", "environment")
	flag.Parse()
	appEnv := getValue(*env, os.Getenv("APP_ENV"), "local")
	logrus.Infof("env: %s", appEnv)
	fmt.Println("bamboo-request-controller")
	liberrors.Errorf("aa")

	cfg, _, sqlDB, tp := initialize(ctx, appEnv)
	defer sqlDB.Close()
	defer tp.ForceFlush(ctx) // flushes any pending spans

	gracefulShutdownTime2 := time.Duration(cfg.Shutdown.TimeSec2) * time.Second

	result := run(ctx, cfg)

	time.Sleep(gracefulShutdownTime2)
	logrus.Info("exited")
	os.Exit(result)
}

func applicationRequestHandler(ctx context.Context, request bamboolib.ApplicationRequest) error {
	// Write a retry information  to DB

	// Send a message to the destination
	return nil
}

func run(ctx context.Context, cfg *config.Config) int {
	var eg *errgroup.Group
	eg, ctx = errgroup.WithContext(ctx)

	eg.Go(func() error {
		return requestReader(ctx, kafka.ReaderConfig{
			Brokers:  []string{"localhost:29092"},
			GroupID:  "consumer-group-id",
			Topic:    "my-topic",
			MaxBytes: 10e6, // 10MB
		}) // nolint:wrapcheck
	})
	eg.Go(func() error {
		<-ctx.Done()
		return ctx.Err() // nolint:wrapcheck
	})

	if err := eg.Wait(); err != nil {
		logrus.Error(err)
		return 1
	}
	return 0
}

func initialize(ctx context.Context, env string) (*config.Config, *gorm.DB, *sql.DB, *sdktrace.TracerProvider) {
	cfg, err := config.LoadConfig(env)
	if err != nil {
		panic(err)
	}

	// init log
	if err := libconfig.InitLog(env, cfg.Log); err != nil {
		panic(err)
	}

	// init tracer
	tp, err := libconfig.InitTracerProvider(cfg.App.Name, cfg.Trace)
	if err != nil {
		panic(err)
	}
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// init db
	db, sqlDB, err := libconfig.InitDB(cfg.DB, sqls.SQL)
	if err != nil {
		panic(err)
	}

	return cfg, db, sqlDB, tp
}
func requestReader(ctx context.Context, kafkaReaderConfig kafka.ReaderConfig) error {
	r := kafka.NewReader(kafkaReaderConfig)

	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			if err := r.Close(); err != nil {
				log.Fatal("failed to close reader:", err)
				return err
			}
		}
		fmt.Printf("message at offset %d: %s = %s\n", m.Offset, string(m.Key), string(m.Value))
	}
}
