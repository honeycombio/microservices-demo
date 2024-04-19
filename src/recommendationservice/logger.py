#!/usr/bin/python
import logging
import sys
from logging.handlers import RotatingFileHandler
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

    return logger
