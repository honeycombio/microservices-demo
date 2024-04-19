#!/usr/bin/python
import os
import random
import time
from concurrent import futures

import grpc

from random import (
    random,
    sample,
)
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

logger = getJSONLogger('recommendationservice-server', log_filename='recommendationservice.log')

worker_pool = futures.ThreadPoolExecutor(max_workers=10)

tracer = trace.get_tracer(__name__)


def get_random_wait_time(max_time, buckets):
    num = 0
    val = max_time / buckets
    for i in range(buckets):
        num += random() * val

    return num


def sleep_random(max_time):
    rnd = get_random_wait_time(max_time, 4)
    time.sleep(rnd / 1000)


def mock_database_call(max_time, name, query):
    with tracer.start_as_current_span(name) as span:
        # span = trace.get_current_span()
        span.set_attribute("db.statement", query)
        span.set_attribute("db.name", "recommendation")
        sleep_random(max_time)


class RecommendationService(demo_pb2_grpc.RecommendationServiceServicer):
    def ListRecommendations(self, request, context):
        with tracer.start_as_current_span("ListRecommendationsFunction"):
            mock_database_call(250,
                               "SELECT recommendation.products",
                               "SELECT * FROM products WHERE category IN (?)")

            span = trace.get_current_span()
            span.set_attribute("app.python.active_threads", len(worker_pool._threads))
            span.set_attribute("app.python.pending_pool", worker_pool._work_queue.qsize())
            max_responses = 5

            # fetch list of products from product catalog stub
            cat_response = product_catalog_stub.ListProducts(demo_pb2.Empty())
            product_ids = [x.id for x in cat_response.products]
            filtered_products = list(set(product_ids) - set(request.product_ids))
            num_products = len(filtered_products)
            num_return = min(max_responses, num_products)

            # sample list of indicies to return
            indices = sample(range(num_products), num_return)

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

    try:
        # Attempt to retrieve environment variables
        service_name = os.environ.get("SERVICE_NAME")
        pod_ip = os.environ.get("POD_IP")

        # Check if the environment variables are provided
        if service_name is None:
            raise ValueError("Environment variable 'SERVICE_NAME' is missing.")
        if pod_ip is None:
            raise ValueError("Environment variable 'POD_IP' is missing.")

        # create Resource attributes used by the OpenTelemetry SDK
        resource = Resource(attributes={
            "service.name": service_name,
            "service.version": "0.1",
            "ip": pod_ip
        })
    except ValueError as ve:
        # Log the specific error and re-raise it
        logger.error(f"Error creating resource: {str(ve)}")
        raise
    except Exception as e:
        # Catch any other unexpected errors
        logger.error("Error creating resource: " + str(e))
        raise
    else:
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
