#!/usr/bin/python
from concurrent import futures
import os
import time
import grpc
from jinja2 import Environment, FileSystemLoader, select_autoescape, TemplateError

import demo_pb2
import demo_pb2_grpc
from grpc_health.v1 import health_pb2
from grpc_health.v1 import health_pb2_grpc

from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.grpc import (
    GrpcInstrumentorServer,
)
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

from random import randint
from time import sleep

from logger import getJSONLogger

logger = getJSONLogger('emailservice-server')

env = Environment(
    loader=FileSystemLoader('templates'),
    autoescape=select_autoescape(['html', 'xml'])
)
template = env.get_template('confirmation.html')


class BaseEmailService(demo_pb2_grpc.EmailServiceServicer):
    def Check(self, request, context):
        return health_pb2.HealthCheckResponse(
            status=health_pb2.HealthCheckResponse.SERVING)


class DummyEmailService(BaseEmailService):
    def SendOrderConfirmation(self, request, context):
        sleep(randint(10, 250) / 1000)

        email = request.email
        order = request.order

        logger.info('A request to send order confirmation email to {} has been received.'.format(email))

        try:
            confirmation = template.render(order=order)
        except TemplateError as err:
            context.set_details("An error occurred when preparing the confirmation mail.")
            logger.error(err.message)
            context.set_code(grpc.StatusCode.INTERNAL)
            return demo_pb2.Empty()

        logger.info(confirmation)

        return demo_pb2.Empty()

    def Watch(self, request, context):
        return health_pb2.HealthCheckResponse(
            status=health_pb2.HealthCheckResponse.UNIMPLEMENTED)


class HealthCheck():
    def Check(self, request, context):
        return health_pb2.HealthCheckResponse(
            status=health_pb2.HealthCheckResponse.SERVING)


def start():
    worker_pool = futures.ThreadPoolExecutor(max_workers=10)
    server = grpc.server(worker_pool)

    service = DummyEmailService()

    demo_pb2_grpc.add_EmailServiceServicer_to_server(service, server)
    health_pb2_grpc.add_HealthServicer_to_server(service, server)

    port = os.environ.get('PORT', "8080")
    logger.info("listening on port: " + port)
    server.add_insecure_port('[::]:' + port)
    server.start()
    try:
        while True:
            time.sleep(3600)
    except KeyboardInterrupt:
        server.stop(0)


if __name__ == '__main__':
    logger.info('starting the email service.')

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

    # Add OpenTelemetry auto-instrumentation hooks for gRPC server communications
    server_instrumentor = GrpcInstrumentorServer().instrument()

    start()
