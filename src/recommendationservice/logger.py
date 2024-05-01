import logging
import os
import sys
from logging.handlers import RotatingFileHandler

from opentelemetry._logs import set_logger_provider
from opentelemetry.exporter.otlp.proto.grpc._log_exporter import \
    OTLPLogExporter
from opentelemetry.sdk._logs import LoggerProvider, LoggingHandler
from opentelemetry.sdk._logs.export import BatchLogRecordProcessor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from pythonjsonlogger import jsonlogger


class CustomJsonFormatter(jsonlogger.JsonFormatter):
    def add_fields(self, log_record, record, message_dict):
        super(CustomJsonFormatter, self).add_fields(log_record, record, message_dict)
        if not log_record.get('timestamp'):
            log_record['timestamp'] = record.created
        if log_record.get('severity'):
            log_record['severity'] = log_record['severity'].upper()
        else:
            log_record['severity'] = record.levelname.upper()
        log_record['name'] = record.name
        log_record['message'] = record.getMessage()


def getJSONLogger(name, log_filename):
    logger = logging.getLogger(name)
    logger.setLevel(logging.INFO)
    logger.propagate = False
    formatter = CustomJsonFormatter('%(timestamp)s %(severity)s %(name)s %(message)s')
    # StreamHandler to output logs to stdout
    console_handler = logging.StreamHandler(sys.stdout)
    console_handler.setFormatter(formatter)
    logger.addHandler(console_handler)

    # RotatingFileHandler for logging into a file
    file_handler = RotatingFileHandler(log_filename, maxBytes=102400)
    file_handler.setFormatter(formatter)
    logger.addHandler(file_handler)

    # OpenTelemetry log exporter
    logger_provider = LoggerProvider()
    # set_logger_provider(logger_provider)
    otlp_log_exporter = OTLPLogExporter(
        endpoint=os.environ.get('OTEL_EXPORTER_OTLP_ENDPOINT'),
        insecure=True)
    logger_provider.add_log_record_processor(BatchLogRecordProcessor(otlp_log_exporter))
    otel_log_handler = LoggingHandler(logger_provider=logger_provider)
    logger.addHandler(otel_log_handler)

    return logger
