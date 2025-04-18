package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	pb "github.com/honeycombio/microservices-demo/src/checkoutservice/demo/msdemo"
	"github.com/honeycombio/microservices-demo/src/checkoutservice/money"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelLog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

const (
	listenPort = "5050"
)

var requestCache = cache.New(5*time.Minute, 10*time.Minute)
var cacheUserThreshold = 35000
var log *logrus.Logger

type OrderCache struct {
	OrderId   string
	UserId    string
	RequestId string
	Currency  string
}

// OtelHook is a logrus hook that sends logs to OpenTelemetry
type OtelHook struct{}

func (h *OtelHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *OtelHook) Fire(entry *logrus.Entry) error {
	logger := global.Logger("logrus-otel")
	attrs := []otelLog.KeyValue{}
	if entry.Data["trace_id"] != nil && entry.Data["span_id"] != nil {
		attrs = []otelLog.KeyValue{
			otelLog.String("trace.trace_id", entry.Data["trace_id"].(string)),
			otelLog.String("trace.parent_id", entry.Data["span_id"].(string)),
			otelLog.String("meta.annotation_type", "span_event"),
		}
	}
	if entry.Data["error"] != nil {
		attrs = append(attrs, otelLog.String("error.message", fmt.Sprintf("%+v", entry.Data["error"])))
	}
	var record otelLog.Record
	record.AddAttributes(attrs...)
	record.SetBody(otelLog.StringValue(entry.Message))
	if entry.Data["error"] != nil {
		record.SetSeverity(otelLog.SeverityError)
	} else {
		record.SetSeverity(otelLog.Severity(entry.Level))
	}
	record.SetTimestamp(entry.Time)
	logger.Emit(context.Background(), record)
	// fmt.Println("** OTELHOOK triggered for log level:", entry.Level)
	return nil
}

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
	log.AddHook(&OtelHook{}) // Otel Hook is added to logrus to send logs to OpenTelemetry
}

type checkoutService struct {
	cartSvcAddr   string
	cartSvcClient pb.CartServiceClient

	currencySvcAddr   string
	currencySvcClient pb.CurrencyServiceClient

	emailSvcAddr   string
	emailSvcClient pb.EmailServiceClient

	paymentSvcAddr   string
	paymentSvcClient pb.PaymentServiceClient

	productCatalogSvcAddr   string
	productCatalogSvcClient pb.ProductCatalogServiceClient

	shippingSvcAddr   string
	shippingSvcClient pb.ShippingServiceClient
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
		semconv.ServiceNameKey.String("checkout"),
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

	// Create the exporter
	exporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		log.Warnf("warn: Failed to create trace exporter: %v", err)
	}

	// Specify the TextMapPropagator to ensure spans propagate across service boundaries
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.Baggage{}, propagation.TraceContext{}))

	// Set standard attributes per semantic conventions
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String("checkout"),
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
	// Initialize OpenTelemetry Log and Tracing
	ctx := context.Background()
	lp := initOtelLogging(ctx)
	defer func() { _ = lp.Shutdown(ctx) }()
	tp := initOtelTracing(ctx, log)
	defer func() { _ = tp.Shutdown(ctx) }()

	cut, err := strconv.Atoi(os.Getenv("CACHE_USER_THRESHOLD"))
	if err == nil {
		cacheUserThreshold = cut
	}

	port := listenPort
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}

	svc := new(checkoutService)
	mustMapEnv(&svc.cartSvcAddr, "CART_SERVICE_ADDR")
	c := mustCreateClientConn(svc.cartSvcAddr)
	svc.cartSvcClient = pb.NewCartServiceClient(c)
	defer c.Close()

	mustMapEnv(&svc.currencySvcAddr, "CURRENCY_SERVICE_ADDR")
	c = mustCreateClientConn(svc.currencySvcAddr)
	svc.currencySvcClient = pb.NewCurrencyServiceClient(c)
	defer c.Close()

	mustMapEnv(&svc.emailSvcAddr, "EMAIL_SERVICE_ADDR")
	c = mustCreateClientConn(svc.emailSvcAddr)
	svc.emailSvcClient = pb.NewEmailServiceClient(c)
	defer c.Close()

	mustMapEnv(&svc.paymentSvcAddr, "PAYMENT_SERVICE_ADDR")
	c = mustCreateClientConn(svc.paymentSvcAddr)
	svc.paymentSvcClient = pb.NewPaymentServiceClient(c)
	defer c.Close()

	mustMapEnv(&svc.productCatalogSvcAddr, "PRODUCT_CATALOG_SERVICE_ADDR")
	c = mustCreateClientConn(svc.productCatalogSvcAddr)
	svc.productCatalogSvcClient = pb.NewProductCatalogServiceClient(c)
	defer c.Close()

	mustMapEnv(&svc.shippingSvcAddr, "SHIPPING_SERVICE_ADDR")
	c = mustCreateClientConn(svc.shippingSvcAddr)
	svc.shippingSvcClient = pb.NewShippingServiceClient(c)
	defer c.Close()

	log.Infof("service config: %+v", svc)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}

	// create gRPC server with OpenTelemetry instrumentation on all incoming requests
	srv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	pb.RegisterCheckoutServiceServer(srv, svc)
	healthpb.RegisterHealthServer(srv, svc)

	log.Infof("starting to listen on tcp: %q", lis.Addr().String())
	err = srv.Serve(lis)
	log.Fatal(err)
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

func (cs *checkoutService) Check(_ context.Context, _ *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (cs *checkoutService) Watch(_ *healthpb.HealthCheckRequest, _ healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}

func (cs *checkoutService) GetCacheSize(_ context.Context, _ *pb.Empty) (*pb.CacheSizeResponse, error) {
	return &pb.CacheSizeResponse{
		CacheSize: int64(requestCache.ItemCount()),
	}, nil
}

func (cs *checkoutService) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.PlaceOrderResponse, error) {
	log.Infof("[PlaceOrder] user_id=%q user_currency=%q", req.UserId, req.UserCurrency)

	orderID, err := uuid.NewUUID()

	var (
		orderIDKey   = attribute.Key("app.order_id")
		userIDKey    = attribute.Key("app.user_id")
		requestIDKey = attribute.Key("app.request_id")
		cachesizeKey = attribute.Key("app.cache_size")
		buildIdKey   = attribute.Key("app.build_id")
	)

	// get userID and requestsID from Tracing Baggage
	bags := baggage.FromContext(ctx)
	userID := bags.Member("app.user_id").Value()
	requestID := bags.Member("app.request_id").Value()
	buildId := bags.Member("app.build_id").Value()

	ordCache := &OrderCache{
		OrderId:   orderID.String(),
		UserId:    userID,
		RequestId: requestID,
		Currency:  req.UserCurrency,
	}

	// Okay we need to fake some problems some how...
	cacheIncrease := determineCacheIncrease(requestCache.ItemCount())
	log.Debugf("increasing cache by: %d", cacheIncrease)
	for i := 0; i < cacheIncrease; i++ {
		requestCache.Set(requestID+strconv.Itoa(i), ordCache, cache.NoExpiration)
	}
	cachesize := requestCache.ItemCount()
	log.Debugf("cachesize: %d", cachesize)

	// Add orderid to Tracing Baggage
	orderIDMember, _ := baggage.NewMember("app.order_id", orderID.String())
	bags, _ = bags.SetMember(orderIDMember)
	ctx = baggage.ContextWithBaggage(ctx, bags)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		cachesizeKey.Int(cachesize),
		userIDKey.String(userID),
		orderIDKey.String(orderID.String()),
		requestIDKey.String(requestID),
		buildIdKey.String(buildId),
	)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate order uuid")
	}

	// Prepare Order
	prep, err := cs.prepareOrderItemsAndShippingQuoteFromCart(ctx, req.UserId, req.UserCurrency, req.Address, cachesize)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	total := pb.Money{CurrencyCode: req.UserCurrency,
		Units: 0,
		Nanos: 0}
	total = money.Must(money.Sum(total, *prep.shippingCostLocalized))
	for _, it := range prep.orderItems {
		multPrice := money.MultiplySlow(*it.Cost, uint32(it.GetItem().GetQuantity()))
		total = money.Must(money.Sum(total, multPrice))
	}
	span.AddEvent("prepared", trace.WithAttributes(
		orderIDKey.String(orderID.String()),
		userIDKey.String(userID),
		attribute.String("currency", req.UserCurrency),
	))

	// Charge Card
	txID, err := cs.chargeCard(ctx, &total, req.CreditCard)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to charge card: %+v", err)
	}
	log.Debugf("payment went through (transaction_id: %s)", txID)
	amt := float64(total.Units) + (float64(total.Nanos) / 100)
	span.AddEvent("charged", trace.WithAttributes(
		orderIDKey.String(orderID.String()),
		userIDKey.String(userID),
		attribute.String("currency", req.UserCurrency),
		attribute.Float64("chargeTotal", amt),
	))

	// Ship Order
	shippingTrackingID, err := cs.shipOrder(ctx, req.Address, prep.cartItems)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "shipping error: %+v", err)
	}

	span.AddEvent("shipped", trace.WithAttributes(
		orderIDKey.String(orderID.String()),
		userIDKey.String(userID),
		attribute.Int("itemCount", len(prep.cartItems)),
		attribute.String("orderItems", fmt.Sprintf("%+v", prep.orderItems)),
	))

	orderResult := &pb.OrderResult{
		OrderId:            orderID.String(),
		ShippingTrackingId: shippingTrackingID,
		ShippingCost:       prep.shippingCostLocalized,
		ShippingAddress:    req.Address,
		Items:              prep.orderItems,
	}

	// empty cart async
	go func() {
		ctx := trace.ContextWithSpan(context.Background(), span)
		err := cs.emptyUserCart(ctx, req.UserId)
		if err != nil {

		}
	}()

	// send order confirmation async
	go func() {
		ctx := trace.ContextWithSpan(context.Background(), span)
		if err := cs.sendOrderConfirmation(ctx, req.Email, orderResult); err != nil {
			log.Warnf("failed to send order confirmation to %q: %+v", req.Email, err)
		} else {
			log.Debugf("order confirmation email sent to %q", req.Email)
		}
	}()

	resp := &pb.PlaceOrderResponse{Order: orderResult}
	return resp, nil
}

type orderPrep struct {
	orderItems            []*pb.OrderItem
	cartItems             []*pb.CartItem
	shippingCostLocalized *pb.Money
}

func determineCacheIncrease(curSize int) int {
	if curSize < cacheUserThreshold {
		// random number between 8 and 9
		return rand.Intn(2) + 8
	} else {
		// we want to go from 9 to 20 (spread of 11) proportionately as we increase to our max of 54000
		max := 54000
		spread := max - cacheUserThreshold
		pos := curSize - cacheUserThreshold
		rndOffset := float64(pos) / float64(spread) * 10

		// proportional increase + base (9) + random offset (which is also proportional to increase)
		return ((pos * 11) / spread) + 9 + rand.Intn(int(rndOffset)+1)
	}
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

func mockDatabaseCall(ctx context.Context, maxTime int, name, query string) {
	tracer := otel.GetTracerProvider().Tracer("")
	ctx, span := tracer.Start(ctx, name)
	span.SetAttributes(attribute.String("db.statement", query),
		attribute.String("db.name", "checkout"))
	defer span.End()

	sleepRandom(maxTime)
}

func loadDiscountFromDatabase(ctx context.Context, cachesize int) string {
	numCalls := math.Max(1, math.Pow(float64(cachesize)/6000, 4)/3000)
	for i := float64(0); i < numCalls; i++ {
		mockDatabaseCall(ctx, 250, "SELECT checkout.discounts", "SELECT * FROM discounts WHERE user = ?")
	}

	discount := rand.Intn(20)
	return strconv.Itoa(discount)
}

func getDiscounts(ctx context.Context, u string, cachesize int) string {
	tracer := otel.GetTracerProvider().Tracer("")
	ctx, span := tracer.Start(ctx, "getDiscounts")
	var (
		userIDKey = attribute.Key("app.user_id")
	)
	span.SetAttributes(userIDKey.String(u))
	defer span.End()
	rnd := rand.Float32()
	if (u == "20109" && rnd < 0.5) || (rnd < 0.25) {
		return loadDiscountFromDatabase(ctx, cachesize)
	} else {
		return loadDiscountFromDatabase(ctx, 0)
	}

}

func (cs *checkoutService) prepareOrderItemsAndShippingQuoteFromCart(ctx context.Context, userID, userCurrency string, address *pb.Address, cachesize int) (orderPrep, error) {
	var out orderPrep
	cartItems, err := cs.getUserCart(ctx, userID)
	if err != nil {
		return out, fmt.Errorf("cart failure: %+v", err)
	}
	orderItems, err := cs.prepOrderItems(ctx, cartItems, userCurrency)
	if err != nil {
		return out, fmt.Errorf("failed to prepare order: %+v", err)
	}

	discount := getDiscounts(ctx, userID, cachesize)
	if discount != "" {
		log.Debugf(fmt.Sprintf("Got a discount: %v.", discount))
	}

	span := trace.SpanFromContext(ctx)
	span.AddEvent("discounted", trace.WithAttributes(
		attribute.String("app.user_id", userID),
		attribute.String("currency", userCurrency),
		attribute.String("discount", discount),
	))

	shippingUSD, err := cs.quoteShipping(ctx, address, cartItems)
	if err != nil {
		return out, fmt.Errorf("shipping quote failure: %+v", err)
	}
	shippingPrice, err := cs.convertCurrency(ctx, shippingUSD, userCurrency)
	if err != nil {
		return out, fmt.Errorf("failed to convert shipping cost to currency: %+v", err)
	}

	out.shippingCostLocalized = shippingPrice
	out.cartItems = cartItems
	out.orderItems = orderItems
	sleepRandom(25)
	return out, nil
}

func (cs *checkoutService) quoteShipping(ctx context.Context, address *pb.Address, items []*pb.CartItem) (*pb.Money, error) {
	shippingQuote, err := cs.shippingSvcClient.
		GetQuote(ctx, &pb.GetQuoteRequest{
			Address: address,
			Items:   items})
	if err != nil {
		return nil, fmt.Errorf("failed to get shipping quote: %+v", err)
	}
	sleepRandom(40)
	return shippingQuote.GetCostUsd(), nil
}

func (cs *checkoutService) getUserCart(ctx context.Context, userID string) ([]*pb.CartItem, error) {
	var (
		userIDKey = attribute.Key("app.user_id")
	)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(userIDKey.String(userID))

	cart, err := cs.cartSvcClient.GetCart(ctx, &pb.GetCartRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("failed to get user cart during checkout: %+v", err)
	}
	sleepRandom(40)
	return cart.GetItems(), nil
}

func (cs *checkoutService) emptyUserCart(ctx context.Context, userID string) error {

	if _, err := cs.cartSvcClient.EmptyCart(ctx, &pb.EmptyCartRequest{UserId: userID}); err != nil {
		return fmt.Errorf("failed to empty user cart during checkout: %+v", err)
	}
	sleepRandom(20)
	return nil
}

func (cs *checkoutService) prepOrderItems(ctx context.Context, items []*pb.CartItem, userCurrency string) ([]*pb.OrderItem, error) {
	out := make([]*pb.OrderItem, len(items))

	for i, item := range items {
		product, err := cs.productCatalogSvcClient.GetProduct(ctx, &pb.GetProductRequest{Id: item.GetProductId()})
		if err != nil {
			return nil, fmt.Errorf("failed to get product #%q", item.GetProductId())
		}
		price, err := cs.convertCurrency(ctx, product.GetPriceUsd(), userCurrency)
		if err != nil {
			return nil, fmt.Errorf("failed to convert price of %q to %s", item.GetProductId(), userCurrency)
		}
		out[i] = &pb.OrderItem{
			Item: item,
			Cost: price}
	}
	sleepRandom(30)
	return out, nil
}

func (cs *checkoutService) convertCurrency(ctx context.Context, from *pb.Money, toCurrency string) (*pb.Money, error) {
	result, err := cs.currencySvcClient.Convert(ctx, &pb.CurrencyConversionRequest{
		From:   from,
		ToCode: toCurrency})
	if err != nil {
		return nil, fmt.Errorf("failed to convert currency: %+v", err)
	}
	sleepRandom(20)
	return result, err
}

func (cs *checkoutService) chargeCard(ctx context.Context, amount *pb.Money, paymentInfo *pb.CreditCardInfo) (string, error) {
	paymentResp, err := cs.paymentSvcClient.Charge(ctx, &pb.ChargeRequest{
		Amount:     amount,
		CreditCard: paymentInfo})
	if err != nil {
		return "", fmt.Errorf("could not charge the card: %+v", err)
	}
	sleepRandom(50)
	return paymentResp.GetTransactionId(), nil
}

func (cs *checkoutService) sendOrderConfirmation(ctx context.Context, email string, order *pb.OrderResult) error {
	_, err := cs.emailSvcClient.SendOrderConfirmation(ctx, &pb.SendOrderConfirmationRequest{
		Email: email,
		Order: order})
	sleepRandom(30)
	return err
}

func (cs *checkoutService) shipOrder(ctx context.Context, address *pb.Address, items []*pb.CartItem) (string, error) {
	resp, err := cs.shippingSvcClient.ShipOrder(ctx, &pb.ShipOrderRequest{
		Address: address,
		Items:   items})
	if err != nil {
		return "", fmt.Errorf("shipment failed: %+v", err)
	}
	sleepRandom(25)
	return resp.GetTrackingId(), nil
}
