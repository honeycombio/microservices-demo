package msdemo;

import com.google.common.collect.ImmutableListMultimap;
import com.google.common.collect.Iterables;
import io.opentelemetry.api.GlobalOpenTelemetry;
import io.opentelemetry.api.common.Attributes;
import io.opentelemetry.api.trace.StatusCode;
import io.opentelemetry.api.trace.Tracer;
import io.opentelemetry.context.Scope;
import msdemo.Demo.Ad;
import msdemo.Demo.AdRequest;
import msdemo.Demo.AdResponse;
import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.StatusRuntimeException;
import io.grpc.health.v1.HealthCheckResponse.ServingStatus;
import io.grpc.protobuf.services.*;
import io.grpc.stub.StreamObserver;

import io.opentelemetry.api.trace.Span;
import io.opentelemetry.api.common.AttributeKey;
import io.opentelemetry.extension.annotations.WithSpan;

import java.io.IOException;
import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import java.util.Random;

import org.apache.logging.log4j.Level;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;


public final class AdService {

    private static final Logger logger = LogManager.getLogger(AdService.class);

    @SuppressWarnings("FieldCanBeLocal")
    private static final int MAX_ADS_TO_SERVE = 2;

    private Server server;
    private HealthStatusManager healthMgr;

    private static final AdService service = new AdService();

    private void start() throws IOException {
        int port = Integer.parseInt(System.getenv().getOrDefault("PORT", "9555"));
        healthMgr = new HealthStatusManager();

        logger.info("Building server on " + port);
        server =
                ServerBuilder.forPort(port)
                        .addService(new AdServiceImpl())
                        .addService(healthMgr.getHealthService())
                        .build()
                        .start();

        logger.info("Ad Service started, listening on " + port);
        Runtime.getRuntime()
                .addShutdownHook(
                        new Thread(
                                () -> {
                                    // Use stderr here since the logger may have been reset by its JVM shutdown hook.
                                    System.err.println(
                                            "*** shutting down gRPC ads server since JVM is shutting down");
                                    AdService.this.stop();
                                    System.err.println("*** server shut down");
                                }));

        healthMgr.setStatus("", ServingStatus.SERVING);
    }

    private static float getRandomWaitTime(int max, int buckets) {
        float num = 0;
        float val = max / (float)buckets;
        for (int i = 0; i < buckets; i++) {
            num += Math.random() * val;
        }
        return num;
    }

    private static void sleepRandom(int max) {
        float rnd = getRandomWaitTime(max, 4);
        try {
            Thread.sleep((long) rnd);
        } catch (InterruptedException e) {
            e.printStackTrace();
        }
    }

    private static void mockDatabaseCall(int maxTime, String name, String query) {
        Tracer tracer = GlobalOpenTelemetry.getTracer("");
        Span span = tracer.spanBuilder(name).startSpan();
        span.makeCurrent();
        span.setAttribute("db.statement", query);
        span.setAttribute("db.name", "ads");
        sleepRandom(maxTime);
        span.end();
    }

    private void stop() {
        if (server != null) {
            healthMgr.clearStatus("");
            server.shutdown();
        }
    }

    private static class AdServiceImpl extends msdemo.AdServiceGrpc.AdServiceImplBase {

        /**
         * Retrieves ads based on context provided in the request {@code AdRequest}.
         *
         * @param req              the request containing context.
         * @param responseObserver the stream observer which gets notified with the value of {@code
         *                         AdResponse}
         */
        @Override
        // Wrap function in an OpenTelemetry span
        @WithSpan  //results in a span name of AdServiceImpl.getAds
        public void getAds(AdRequest req, StreamObserver<AdResponse> responseObserver) {
            AdService service = AdService.getInstance();

            // Get the current OpenTelemetry span
            Span span = Span.current();
            try {
                // Add a span attribute
                span.setAttribute("method", "getAds");
                span.setAttribute("context_keys", req.getContextKeysList().toString());

                mockDatabaseCall(125, "SELECT ads.ads", "SELECT * from ads WHERE context_words LIKE ?");

                List<Ad> allAds = new ArrayList<>();
                logger.info("received ad request (context_words=" + req.getContextKeysList() + ")");

                long keyCount = req.getContextKeysCount();
                if (req.getContextKeysCount() > 0) {

                    // Add a span event
                    span.addEvent(
                            "Constructing Ads using context",
                            io.opentelemetry.api.common.Attributes.of(
                                    AttributeKey.stringKey("Context Keys"), req.getContextKeysList().toString(),
                                    AttributeKey.longKey("Context Keys length"), keyCount));

                    for (int i = 0; i < keyCount; i++) {
                        Collection<Ad> ads = service.getAdsByCategory(req.getContextKeys(i));
                        allAds.addAll(ads);
                    }

                } else {
                    span.addEvent("No Context provided. Constructing random Ads.");
                    allAds = service.getRandomAds();
                }

                if (allAds.isEmpty()) {
                    span.addEvent("No Ads found based on context. Constructing random Ads.");
                    allAds = service.getRandomAds();
                }

                AdResponse reply = AdResponse.newBuilder().addAllAds(allAds).build();
                responseObserver.onNext(reply);
                responseObserver.onCompleted();

            } catch (StatusRuntimeException e) {
                logger.log(Level.WARN, "GetAds Failed with status {}", e.getStatus());
                span.setStatus(StatusCode.ERROR);
                responseObserver.onError(e);

            }
        }
    }

    private static final ImmutableListMultimap<String, Ad> adsMap = createAdsMap();

    private Collection<Ad> getAdsByCategory(String category) {
        return adsMap.get(category);
    }

    private static final Random random = new Random();

    // Wrap function in an OpenTelemetry Span
    @WithSpan("random-ads")
    private List<Ad> getRandomAds() {
        List<Ad> ads = new ArrayList<>(MAX_ADS_TO_SERVE);
        Collection<Ad> allAds = adsMap.values();

        for (int i = 0; i < MAX_ADS_TO_SERVE; i++) {
            ads.add(Iterables.get(allAds, random.nextInt(allAds.size())));
        }

        return ads;
    }

    private static AdService getInstance() {
        return service;
    }

    /**
     * Await termination on the main thread since the grpc library uses daemon threads.
     */
    private void blockUntilShutdown() throws InterruptedException {
        if (server != null) {
            server.awaitTermination();
        }
    }

    private static ImmutableListMultimap<String, Ad> createAdsMap() {
        Ad camera =
                Ad.newBuilder()
                        .setRedirectUrl("/product/2ZYFJ3GM2N")
                        .setText("Film camera for sale. 50% off.")
                        .build();
        Ad lens =
                Ad.newBuilder()
                        .setRedirectUrl("/product/66VCHSJNUP")
                        .setText("Vintage camera lens for sale. 20% off.")
                        .build();
        Ad recordPlayer =
                Ad.newBuilder()
                        .setRedirectUrl("/product/0PUK6V6EV0")
                        .setText("Vintage record player for sale. 30% off.")
                        .build();
        Ad bike =
                Ad.newBuilder()
                        .setRedirectUrl("/product/9SIQT8TOJO")
                        .setText("City Bike for sale. 10% off.")
                        .build();
        Ad baristaKit =
                Ad.newBuilder()
                        .setRedirectUrl("/product/1YMWWN1N4O")
                        .setText("Home Barista kitchen kit for sale. Buy one, get second kit for free")
                        .build();
        Ad airPlant =
                Ad.newBuilder()
                        .setRedirectUrl("/product/6E92ZMYYFZ")
                        .setText("Air plants for sale. Buy two, get third one for free")
                        .build();
        Ad terrarium =
                Ad.newBuilder()
                        .setRedirectUrl("/product/L9ECAV7KIM")
                        .setText("Terrarium for sale. Buy one, get second one for free")
                        .build();
        return ImmutableListMultimap.<String, Ad>builder()
                .putAll("photography", camera, lens)
                .putAll("vintage", camera, lens, recordPlayer)
                .put("cycling", bike)
                .put("cookware", baristaKit)
                .putAll("gardening", airPlant, terrarium)
                .build();
    }

    /**
     * Main launches the server from the command line.
     */
    public static void main(String[] args) throws IOException, InterruptedException {
        logger.info("AdService starting.");
        final AdService service = AdService.getInstance();
        service.start();
        service.blockUntilShutdown();
    }
}
