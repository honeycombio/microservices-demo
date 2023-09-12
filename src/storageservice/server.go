package main

import (
	"context"
	"fmt"
	pb "github.com/honeycombio/microservices-demo/src/checkoutservice/demo/msdemo"
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
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"math"
	"math/rand"
	"net"
	"os"
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
		semconv.ServiceNameKey.String("storage"),
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

	port := "6381"
	log.Infof("starting grpc server at :%s", port)
	run(port)
	select {}
}

func run(port string) string {
	l, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}

	// create gRPC server with OpenTelemetry instrumentation on all incoming requests
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
	)

	svc := &storageService{}

	pb.RegisterStorageServiceServer(srv, svc)
	go func() {
		_ = srv.Serve(l)
	}()
	return l.Addr().String()
}

type storageService struct{}

func (p *storageService) SaveAudit(ctx context.Context, req *pb.SaveAuditRequest) (*pb.Empty, error) {
	log.Infof("saving audit: %s", req.GetId())

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("audit_id", req.GetId()))

	if req.Id == "130720" {
		memoryLeak()
	}

	mockDatabaseCall(ctx, 50, "INSERT audit transaction", fmt.Sprintf("INSERT INTO audit.transactions (audit_id, success) VALUES (%v, 1)", req.Id))

	sleepRandom(1000)

	return &pb.Empty{}, nil
}

func mockDatabaseCall(ctx context.Context, maxTime int, name, query string) {
	tracer := otel.GetTracerProvider().Tracer("")
	ctx, span := tracer.Start(ctx, name)
	span.SetAttributes(attribute.String("db.statement", query),
		attribute.String("db.name", "storage"))
	defer span.End()

	sleepRandom(maxTime)
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

func memoryLeak() {
	s1 := make([]string, math.MaxInt32)
	for {
		//time.Sleep(1 * time.Microsecond)
		s1 = append(s1, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
		log.Infof("len :%d", len(s1))
	}
}
