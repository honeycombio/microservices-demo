#!/usr/bin/python
import os
import random
import time
from concurrent import futures

import grpc

from random import randint
from time import sleep

import demo_pb2
import demo_pb2_grpc
from grpc_health.v1 import health_pb2
from grpc_health.v1 import health_pb2_grpc

from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.grpc import (
    GrpcInstrumentorClient,
    GrpcInstrumentorServer,
)
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from logger import getJSONLogger

logger = getJSONLogger('recommendationservice-server')

worker_pool = futures.ThreadPoolExecutor(max_workers=10)


class RecommendationService(demo_pb2_grpc.RecommendationServiceServicer):
    def ListRecommendations(self, request, context):
        tracer = trace.get_tracer(__name__)
        with tracer.start_as_current_span("ListRecommendationsFunction"):
            sleep(randint(10, 250) / 1000)

            span = trace.get_current_span()
            span.set_attribute("active_threads", len(worker_pool._threads))
            span.set_attribute("pending_pool", worker_pool._work_queue.qsize())
            max_responses = 5

            # fetch list of products from product catalog stub
            cat_response = product_catalog_stub.ListProducts(demo_pb2.Empty())
            product_ids = [x.id for x in cat_response.products]
            filtered_products = list(set(product_ids) - set(request.product_ids))
            num_products = len(filtered_products)
            num_return = min(max_responses, num_products)

            # sample list of indicies to return
            indices = random.sample(range(num_products), num_return)

            # fetch product ids from indices
            prod_list = [filtered_products[i] for i in indices]
            logger.info("[Recv ListRecommendations] product_ids={}".format(prod_list))

            # build and return response
            response = demo_pb2.ListRecommendationsResponse()
            response.product_ids.extend(prod_list)

            return response

    def Check(self, request, context):
        return health_pb2.HealthCheckResponse(
            status=health_pb2.HealthCheckResponse.SERVING)

    def Watch(self, request, context):
        return health_pb2.HealthCheckResponse(
            status=health_pb2.HealthCheckResponse.UNIMPLEMENTED)


if __name__ == "__main__":
    logger.info("initializing recommendationservice")

    # create Resource attributes used by the OpenTelemetry SDK
    resource = Resource(attributes={
        "service.name": os.environ.get("SERVICE_NAME"),
        "service.version": "0.1", "ip": os.environ.get('POD_IP')
    })

    # create the OTLP exporter to send data an insecure OpenTelemetry Collector
    otlp_exporter = OTLPSpanExporter(
        endpoint=os.environ.get('OTEL_EXPORTER_OTLP_ENDPOINT'),
        insecure=True
    )

    # create a Trace Provider
    trace_provider = TracerProvider(resource=resource)
    trace_provider.add_span_processor(
        BatchSpanProcessor(otlp_exporter)
    )

    # set the Trace Provider to be used by the OpenTelemetry SDK
    trace.set_tracer_provider(trace_provider)

    # Add OpenTelemetry auto-instrumentation hooks for gRPC client and server communications
    client_instrumentor = GrpcInstrumentorClient().instrument()
    server_instrumentor = GrpcInstrumentorServer().instrument()

    catalog_addr = os.environ.get('PRODUCT_CATALOG_SERVICE_ADDR', '')
    if catalog_addr == "":
        raise Exception('PRODUCT_CATALOG_SERVICE_ADDR environment variable not set')
    logger.info("product catalog address: " + catalog_addr)
    channel = grpc.insecure_channel(catalog_addr)
    product_catalog_stub = demo_pb2_grpc.ProductCatalogServiceStub(channel)

    server = grpc.server(worker_pool)

    # add class to gRPC server
    service = RecommendationService()
    demo_pb2_grpc.add_RecommendationServiceServicer_to_server(service, server)
    health_pb2_grpc.add_HealthServicer_to_server(service, server)

    # start server
    port = os.environ.get('PORT', "8080")
    logger.info("listening on port: " + port)
    server.add_insecure_port('[::]:' + port)
    server.start()

    # keep alive
    try:
        while True:
            time.sleep(10000)
    except KeyboardInterrupt:
        server.stop(0)
