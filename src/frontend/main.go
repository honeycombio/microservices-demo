package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	pb "github.com/honeycombio/microservices-demo/src/frontend/demo/msdemo"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	middleware "go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	port            = "8080"
	defaultCurrency = "USD"
	cookieMaxAge    = 5 //60 * 60 * 48

	cookiePrefix    = "shop_"
	cookieSessionID = cookiePrefix + "session-id"
	cookieCurrency  = cookiePrefix + "currency"
)

var (
	whitelistedCurrencies = map[string]bool{
		"USD": true,
		"EUR": true,
		"CAD": true,
		"JPY": true,
		"GBP": true,
		"TRY": true}
)

type ctxKeySessionID struct{}

type frontendServer struct {
	adSvcAddr   string
	adSvcClient pb.AdServiceClient

	checkoutSvcAddr   string
	checkoutSvcClient pb.CheckoutServiceClient
	getCacheClient    pb.CheckoutServiceClient

	cartSvcAddr   string
	cartSvcClient pb.CartServiceClient

	currencySvcAddr   string
	currencySvcClient pb.CurrencyServiceClient

	productCatalogSvcAddr   string
	productCatalogSvcClient pb.ProductCatalogServiceClient

	recommendationSvcAddr   string
	recommendationSvcClient pb.RecommendationServiceClient

	shippingSvcAddr   string
	shippingSvcClient pb.ShippingServiceClient
}

var CacheTrack *CacheTracker
var PercentNormal = 75
var CacheUserThreshold = 35000
var CacheMarkerThreshold = 30000
var MockBuildId = ""

func main() {

	ctx := context.Background()
	// initialize otel logging first
	lp := initOtelLogging(context.Background())
	defer func() { _ = lp.Shutdown(ctx) }()

	log := logrus.New()
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
	log.AddHook(&OtelHook{})

	// Initialize OpenTelemetry Tracing
	tp := initOtelTracing(ctx, log)
	defer func() { _ = tp.Shutdown(ctx) }()

	p, err := strconv.Atoi(os.Getenv("PERCENT_NORMAL"))
	if err == nil {
		PercentNormal = p
	}
	cut, err := strconv.Atoi(os.Getenv("CACHE_USER_THRESHOLD"))
	if err == nil {
		CacheUserThreshold = cut
	}
	cmt, err := strconv.Atoi(os.Getenv("CACHE_MARKER_THRESHOLD"))
	if err == nil {
		CacheMarkerThreshold = cmt
	}
	apiKey := os.Getenv("HONEYCOMB_API_KEY")
	CacheTrack = NewCacheTracker(CacheUserThreshold, CacheMarkerThreshold, apiKey, log)

	srvPort := port
	if os.Getenv("PORT") != "" {
		srvPort = os.Getenv("PORT")
	}

	addr := os.Getenv("LISTEN_ADDR")

	MockBuildId = randomHex(4)

	svc := new(frontendServer)
	mustMapEnv(&svc.adSvcAddr, "AD_SERVICE_ADDR")
	c := mustCreateClientConn(svc.adSvcAddr)
	svc.adSvcClient = pb.NewAdServiceClient(c)
	defer c.Close()

	mustMapEnv(&svc.cartSvcAddr, "CART_SERVICE_ADDR")
	c = mustCreateClientConn(svc.cartSvcAddr)
	svc.cartSvcClient = pb.NewCartServiceClient(c)
	defer c.Close()

	mustMapEnv(&svc.checkoutSvcAddr, "CHECKOUT_SERVICE_ADDR")
	c = mustCreateClientConn(svc.checkoutSvcAddr)
	svc.checkoutSvcClient = pb.NewCheckoutServiceClient(c)
	defer c.Close()

	mustMapEnv(&svc.currencySvcAddr, "CURRENCY_SERVICE_ADDR")
	c = mustCreateClientConn(svc.currencySvcAddr)
	svc.currencySvcClient = pb.NewCurrencyServiceClient(c)
	defer c.Close()

	mustMapEnv(&svc.productCatalogSvcAddr, "PRODUCT_CATALOG_SERVICE_ADDR")
	c = mustCreateClientConn(svc.productCatalogSvcAddr)
	svc.productCatalogSvcClient = pb.NewProductCatalogServiceClient(c)
	defer c.Close()

	mustMapEnv(&svc.recommendationSvcAddr, "RECOMMENDATION_SERVICE_ADDR")
	c = mustCreateClientConn(svc.recommendationSvcAddr)
	svc.recommendationSvcClient = pb.NewRecommendationServiceClient(c)
	defer c.Close()

	mustMapEnv(&svc.shippingSvcAddr, "SHIPPING_SERVICE_ADDR")
	c = mustCreateClientConn(svc.shippingSvcAddr)
	svc.shippingSvcClient = pb.NewShippingServiceClient(c)
	defer c.Close()

	// getCache connection is not instrumented
	conn, err := grpc.DialContext(ctx, svc.checkoutSvcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		panic(errors.Wrapf(err, "grpc: failed to create getCache connection to checkout service %s", svc.checkoutSvcAddr))
	}
	svc.getCacheClient = pb.NewCheckoutServiceClient(conn)

	r := mux.NewRouter()

	r.HandleFunc("/", instrumentHandler(svc.homeHandler)).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/product/{id}", instrumentHandler(svc.productHandler)).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/cart", instrumentHandler(svc.viewCartHandler)).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/cart", instrumentHandler(svc.addToCartHandler)).Methods(http.MethodPost)
	r.HandleFunc("/cart/empty", instrumentHandler(svc.emptyCartHandler)).Methods(http.MethodPost)
	r.HandleFunc("/setCurrency", instrumentHandler(svc.setCurrencyHandler)).Methods(http.MethodPost)
	r.HandleFunc("/logout", instrumentHandler(svc.logoutHandler)).Methods(http.MethodGet)
	r.HandleFunc("/cart/checkout", instrumentHandler(svc.placeOrderHandler)).Methods(http.MethodPost)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.FileServer(http.Dir("./dist/"))))
	r.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, "User-agent: *\nDisallow: /") })
	r.HandleFunc("/_healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, "ok") })

	// Add OpenTelemetry instrumentation to incoming HTTP requests controlled by the gorilla/mux Router.
	r.Use(middleware.Middleware("frontend"))

	var handler http.Handler = r
	handler = &logHandler{log: log, next: handler} // add logging
	handler = ensureSessionID(handler)             // add session ID

	CacheTrack.Track(ctx, svc)

	log.Infof("starting server on " + addr + ":" + srvPort)
	log.Fatal(http.ListenAndServe(addr+":"+srvPort, handler))
}

func initOtelLogging(ctx context.Context) *sdklog.LoggerProvider {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "opentelemetry-collector:4317"
	}
	logExporter, err := otlploggrpc.New(
		ctx,
		otlploggrpc.WithEndpoint(endpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatal(err)
	}
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String("frontend"),
	)
	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(
			sdklog.NewBatchProcessor(logExporter),
		),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(provider)
	return provider
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
		semconv.ServiceNameKey.String("frontend"),
	)

	// Create and set the TraceProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp
}

func mustMapEnv(target *string, envKey string) {
	v := os.Getenv(envKey)
	if v == "" {
		panic(fmt.Sprintf("environment variable %q not set", envKey))
	}
	*target = v
}

func mustCreateClientConn(svcAddr string) *grpc.ClientConn {
	c, err := grpc.NewClient(svcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		log.Fatalf("could not connect to %s service, err: %+v", svcAddr, err)
	}

	return c
}

func randomHex(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}
