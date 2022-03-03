package main

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	pb "github.com/honeycombio/microservices-demo/src/checkoutservice/demo/msdemo"
	"github.com/honeycombio/microservices-demo/src/checkoutservice/money"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"math"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"
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

type checkoutService struct {
	productCatalogSvcAddr string
	cartSvcAddr           string
	currencySvcAddr       string
	shippingSvcAddr       string
	emailSvcAddr          string
	paymentSvcAddr        string
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
	// Initialize OpenTelemetry Tracing
	ctx := context.Background()
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
	mustMapEnv(&svc.shippingSvcAddr, "SHIPPING_SERVICE_ADDR")
	mustMapEnv(&svc.productCatalogSvcAddr, "PRODUCT_CATALOG_SERVICE_ADDR")
	mustMapEnv(&svc.cartSvcAddr, "CART_SERVICE_ADDR")
	mustMapEnv(&svc.currencySvcAddr, "CURRENCY_SERVICE_ADDR")
	mustMapEnv(&svc.emailSvcAddr, "EMAIL_SERVICE_ADDR")
	mustMapEnv(&svc.paymentSvcAddr, "PAYMENT_SERVICE_ADDR")

	log.Infof("service config: %+v", svc)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}

	// create gRPC server with OpenTelemetry instrumentation on all incoming requests
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
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
		orderIDKey   = attribute.Key("orderid")
		userIDKey    = attribute.Key("userid")
		requestIDKey = attribute.Key("requestID")
		cachesizeKey = attribute.Key("cachesize")
	)

	// get userID and requestsID from Tracing Baggage
	bags := baggage.FromContext(ctx)
	userID := bags.Member("userid").Value()
	requestID := bags.Member("requestID").Value()

	ordCache := &OrderCache{
		OrderId:   orderID.String(),
		UserId:    userID,
		RequestId: requestID,
		Currency:  req.UserCurrency,
	}

	// Okay we need to fake some problems some how...
	cacheIncrease := determineCacheIncrease(requestCache.ItemCount())
	for i := 0; i < cacheIncrease; i++ {
		requestCache.Set(requestID+strconv.Itoa(i), ordCache, cache.NoExpiration)
	}
	cachesize := requestCache.ItemCount()

	// Add orderid to Tracing Baggage
	orderIDMember, _ := baggage.NewMember("orderid", orderID.String())
	bags, _ = bags.SetMember(orderIDMember)
	ctx = baggage.ContextWithBaggage(ctx, bags)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		cachesizeKey.Int(cachesize),
		userIDKey.String(userID),
		orderIDKey.String(orderID.String()),
		requestIDKey.String(requestID),
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
	log.Infof("payment went through (transaction_id: %s)", txID)
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
			log.Infof("order confirmation email sent to %q", req.Email)
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
	numCalls := math.Max(1, math.Pow(float64(cachesize)/6000, 4)/400)
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
		userIDKey = attribute.Key("userid")
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
		log.Infof(fmt.Sprintf("Got a discount: %v.", discount))
	}

	span := trace.SpanFromContext(ctx)
	span.AddEvent("discounted", trace.WithAttributes(
		attribute.String("userId", userID),
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
	// add OpenTelemetry instrumentation to outgoing gRPC requests
	conn, err := grpc.DialContext(ctx, cs.shippingSvcAddr,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))))

	if err != nil {
		return nil, fmt.Errorf("could not connect shipping service: %+v", err)
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)

	shippingQuote, err := pb.NewShippingServiceClient(conn).
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
	// add OpenTelemetry instrumentation to outgoing gRPC requests
	conn, err := grpc.DialContext(ctx, cs.cartSvcAddr, grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))))

	var (
		userIDKey = attribute.Key("userid")
	)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(userIDKey.String(userID))

	if err != nil {
		return nil, fmt.Errorf("could not connect cart service: %+v", err)
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)

	cart, err := pb.NewCartServiceClient(conn).GetCart(ctx, &pb.GetCartRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("failed to get user cart during checkout: %+v", err)
	}
	sleepRandom(40)
	return cart.GetItems(), nil
}

func (cs *checkoutService) emptyUserCart(ctx context.Context, userID string) error {
	// add OpenTelemetry instrumentation to outgoing gRPC requests
	conn, err := grpc.DialContext(ctx, cs.cartSvcAddr, grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))))

	if err != nil {
		return fmt.Errorf("could not connect cart service: %+v", err)
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)

	if _, err = pb.NewCartServiceClient(conn).EmptyCart(ctx, &pb.EmptyCartRequest{UserId: userID}); err != nil {
		return fmt.Errorf("failed to empty user cart during checkout: %+v", err)
	}
	sleepRandom(20)
	return nil
}

func (cs *checkoutService) prepOrderItems(ctx context.Context, items []*pb.CartItem, userCurrency string) ([]*pb.OrderItem, error) {
	// add OpenTelemetry instrumentation to outgoing gRPC requests
	conn, err := grpc.DialContext(ctx, cs.productCatalogSvcAddr, grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))))

	out := make([]*pb.OrderItem, len(items))

	if err != nil {
		return nil, fmt.Errorf("could not connect product catalog service: %+v", err)
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)
	cl := pb.NewProductCatalogServiceClient(conn)

	for i, item := range items {
		product, err := cl.GetProduct(ctx, &pb.GetProductRequest{Id: item.GetProductId()})
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
	// add OpenTelemetry instrumentation to outgoing gRPC requests
	conn, err := grpc.DialContext(ctx, cs.currencySvcAddr, grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))))

	if err != nil {
		return nil, fmt.Errorf("could not connect currency service: %+v", err)
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)
	result, err := pb.NewCurrencyServiceClient(conn).Convert(ctx, &pb.CurrencyConversionRequest{
		From:   from,
		ToCode: toCurrency})
	if err != nil {
		return nil, fmt.Errorf("failed to convert currency: %+v", err)
	}
	sleepRandom(20)
	return result, err
}

func (cs *checkoutService) chargeCard(ctx context.Context, amount *pb.Money, paymentInfo *pb.CreditCardInfo) (string, error) {
	// add OpenTelemetry instrumentation to outgoing gRPC requests
	conn, err := grpc.DialContext(ctx, cs.paymentSvcAddr, grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))))

	if err != nil {
		return "", fmt.Errorf("failed to connect payment service: %+v", err)
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)

	paymentResp, err := pb.NewPaymentServiceClient(conn).Charge(ctx, &pb.ChargeRequest{
		Amount:     amount,
		CreditCard: paymentInfo})
	if err != nil {
		return "", fmt.Errorf("could not charge the card: %+v", err)
	}
	sleepRandom(50)
	return paymentResp.GetTransactionId(), nil
}

func (cs *checkoutService) sendOrderConfirmation(ctx context.Context, email string, order *pb.OrderResult) error {
	// add OpenTelemetry instrumentation to outgoing gRPC requests
	conn, err := grpc.DialContext(ctx, cs.emailSvcAddr, grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))))

	if err != nil {
		return fmt.Errorf("failed to connect email service: %+v", err)
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)
	_, err = pb.NewEmailServiceClient(conn).SendOrderConfirmation(ctx, &pb.SendOrderConfirmationRequest{
		Email: email,
		Order: order})
	sleepRandom(30)
	return err
}

func (cs *checkoutService) shipOrder(ctx context.Context, address *pb.Address, items []*pb.CartItem) (string, error) {
	// add OpenTelemetry instrumentation to outgoing gRPC requests
	conn, err := grpc.DialContext(ctx, cs.shippingSvcAddr, grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))))

	if err != nil {
		return "", fmt.Errorf("failed to connect email service: %+v", err)
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)
	resp, err := pb.NewShippingServiceClient(conn).ShipOrder(ctx, &pb.ShipOrderRequest{
		Address: address,
		Items:   items})
	if err != nil {
		return "", fmt.Errorf("shipment failed: %+v", err)
	}
	sleepRandom(25)
	return resp.GetTrackingId(), nil
}
