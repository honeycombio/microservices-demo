package main

import (
	"context"
	pb "github.com/honeycombio/microservices-demo/src/checkoutservice/demo/msdemo"
	"github.com/mroth/jitter"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"google.golang.org/grpc"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"
)

var log *logrus.Logger

func init() {
	log = logrus.New()
	log.Level = logrus.DebugLevel
	log.Formatter = &logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
		},
		TimestampFormat: time.RFC3339Nano,
	}
	log.Out = os.Stdout
}

type auditService struct {
}

func initOtelTracing(ctx context.Context, log logrus.FieldLogger) *sdktrace.TracerProvider {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "opentelemetry-collector:4317"
	}

	// Set GRPC options to establish an insecure connection to an OpenTelemetry Collector
	// To establish a TLS connection to a secured endpoint use:
	//   otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	}

	// Create the exporter
	exporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
	if err != nil {
		log.Fatal(err)
	}

	// Specify the TextMapPropagator to ensure spans propagate across service boundaries
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.Baggage{}, propagation.TraceContext{}))

	// Set standard attributes per semantic conventions
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String("audit"),
	)

	// Create and set the TraceProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp
}

func main() {
	// Initialize OpenTelemetry Tracing
	ctx := context.Background()
	tp := initOtelTracing(ctx, log)
	defer func() { _ = tp.Shutdown(ctx) }()

	t := jitter.NewTicker(time.Minute*20, 0.5)
	for range time.Tick(time.Second) {
		auditId := strconv.Itoa(rand.Intn(25) + (rand.Intn(25) * 100) + (rand.Intn(25) * 10000))
		select {
		case <-t.C:
			auditId = "130720"
		default:
		}
		go audit(ctx, auditId)
	}
}

func audit(ctx context.Context, auditId string) {
	tracer := otel.GetTracerProvider().Tracer("")
	ctx, span := tracer.Start(ctx, "audit start")
	defer span.End()

	auditIDKey := attribute.Key("audit_id")
	span.SetAttributes(auditIDKey.String(auditId))

	wg := sync.WaitGroup{}

	if rand.Intn(2) == 0 {
		wg.Add(1)
		go auditProductCatalogService(ctx, auditId, &wg)
	}

	if rand.Intn(2) == 0 {
		wg.Add(1)
		go auditCheckoutService(ctx, auditId, &wg)
	}

	if rand.Intn(2) == 0 {
		wg.Add(1)
		go auditShippingService(ctx, auditId, &wg)
	}

	wg.Wait()

	saveAudit(ctx, auditId)
}

func auditProductCatalogService(ctx context.Context, auditId string, wg *sync.WaitGroup) {
	defer wg.Done()

	// add OpenTelemetry instrumentation to outgoing gRPC requests
	conn, err := grpc.DialContext(ctx, "productcatalogservice:3550", grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithTimeout(time.Second*30))

	if err != nil {
		log.Error("could not connect to product catalog service: %+v", err)
		return
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)

	_, err = pb.NewProductCatalogServiceClient(conn).AuditProductService(ctx, &pb.AuditRequest{Id: auditId})
	if err != nil {
		log.Error("failed to audit product catalog service: %+v", err)
		return
	}

	sleepRandom(1000)
}

func auditCheckoutService(ctx context.Context, auditId string, wg *sync.WaitGroup) {
	defer wg.Done()

	// add OpenTelemetry instrumentation to outgoing gRPC requests
	conn, err := grpc.DialContext(ctx, "checkoutservice:5050", grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithTimeout(time.Second*30))

	if err != nil {
		log.Error("could not connect to checkout service: %+v", err)
		return
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)

	_, err = pb.NewCheckoutServiceClient(conn).AuditCheckoutService(ctx, &pb.AuditRequest{Id: auditId})
	if err != nil {
		log.Error("failed to audit checkout service: %+v", err)
		return
	}

	sleepRandom(1000)
}

func auditShippingService(ctx context.Context, auditId string, wg *sync.WaitGroup) {
	defer wg.Done()

	// add OpenTelemetry instrumentation to outgoing gRPC requests
	conn, err := grpc.DialContext(ctx, "shippingservice:50051", grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithTimeout(time.Second*30))

	if err != nil {
		log.Error("could not connect to shipping service: %+v", err)
		return
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)

	_, err = pb.NewShippingServiceClient(conn).AuditShippingService(ctx, &pb.AuditRequest{Id: auditId})
	if err != nil {
		log.Error("failed to audit shipping service: %+v", err)
		return
	}

	sleepRandom(1000)
}

func saveAudit(ctx context.Context, auditId string) {
	// add OpenTelemetry instrumentation to outgoing gRPC requests
	conn, err := grpc.DialContext(ctx, "storageservice:6381", grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithTimeout(time.Second*30))

	if err != nil {
		log.Error("could not connect to storage service: %+v", err)
		return
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)

	_, err = pb.NewStorageServiceClient(conn).SaveAudit(ctx, &pb.SaveAuditRequest{Id: auditId})
	if err != nil {
		log.Error("failed to save audit transaction: %+v", err)
		return
	}

	sleepRandom(1000)
}

func getRandomWaitTime(max int, buckets int) float32 {
	num := float32(0)
	val := float32(max / buckets)
	for i := 0; i < buckets; i++ {
		num += rand.Float32() * val
	}
	return num
}

func sleepRandom(max int) {
	rnd := getRandomWaitTime(max, 4)
	time.Sleep((time.Duration(rnd)) * time.Millisecond)
}
