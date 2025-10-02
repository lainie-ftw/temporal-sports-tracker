package sports

import (
	"context"
	"crypto/tls"
	"log/slog"
	"os"
	"github.com/joho/godotenv"

	"go.temporal.io/sdk/client"
	tlog "go.temporal.io/sdk/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func GetClientOptions() client.Options {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	err := godotenv.Load()
	if err != nil {
		logger.Warn("No .env file found, relying on environment variables")
	}	

	TemporalAddress := os.Getenv("TEMPORAL_HOST")
	if TemporalAddress == "" {
		slog.Error("TEMPORAL_HOST environment variable is not set")
		os.Exit(1)
	}

	TemporalNamespace := os.Getenv("TEMPORAL_NAMESPACE")
	if TemporalNamespace == "" {
		slog.Error("TEMPORAL_NAMESPACE environment variable is not set")
		os.Exit(1)
	}

	clientOptions := client.Options{
		HostPort:  TemporalAddress,
		Namespace: TemporalNamespace,
		Logger:    tlog.NewStructuredLogger(logger),
	}

	clientOptions.ConnectionOptions = client.ConnectionOptions{
		TLS: &tls.Config{},
		DialOptions: []grpc.DialOption{
			grpc.WithUnaryInterceptor(
				func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
					return invoker(
						metadata.AppendToOutgoingContext(ctx, "temporal-namespace", TemporalNamespace),
						method,
						req,
						reply,
						cc,
						opts...,
					)
				},
			),
		},
	}

	if TemporalAddress != "localhost:7233" && TemporalAddress != "host.docker.internal:7233"{
		TemporalAPIKey := os.Getenv("TEMPORAL_API_KEY")
		if TemporalAPIKey == "" {
			slog.Error("TEMPORAL_API_KEY environment variable is not set")
			os.Exit(1)
		}

		clientOptions.Credentials = client.NewAPIKeyStaticCredentials(TemporalAPIKey)
	} else {
		clientOptions.ConnectionOptions.TLS = nil // Disable TLS for local development
	}

	return clientOptions
}
