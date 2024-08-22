#!/usr/bin/python
import logging
import sys
from pythonjsonlogger import jsonlogger


class CustomJsonFormatter(jsonlogger.JsonFormatter):
    def add_fields(self, log_record, record, message_dict):
        super(CustomJsonFormatter, self).add_fields(log_record, record, message_dict)
        if not log_record.get('timestamp'):
            log_record['timestamp'] = record.created
        if log_record.get('severity'):
            log_record['severity'] = log_record['severity'].upper()
        else:
            log_record['severity'] = record.levelname


def getJSONLogger(name):
    logger = logging.getLogger(name)
    handler = logging.StreamHandler(sys.stdout)
    # Pass the format as a format string with curly braces
    formatter = CustomJsonFormatter(fmt='%(timestamp)s %(severity)s %(name)s %(message)s')
    handler.setFormatter(formatter)
    logger.addHandler(handler)
    logger.setLevel(logging.INFO)
    logger.propagate = False
    return logger
