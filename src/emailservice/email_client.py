#!/usr/bin/python
import grpc

import demo_pb2
import demo_pb2_grpc

from logger import getJSONLogger

logger = getJSONLogger('emailservice-client')


def send_confirmation_email(email, order):
    channel = grpc.insecure_channel('0.0.0.0:8080')
    # channel = grpc.intercept_channel(channel)
    stub = demo_pb2_grpc.EmailServiceStub(channel)

    try:
        response = stub.SendOrderConfirmation(demo_pb2.SendOrderConfirmationRequest(
            email=email,
            order=order
        ))
        logger.info('Request sent.')
    except grpc.RpcError as err:
        logger.error(err.details())
        logger.error('{}, {}'.format(err.code().name, err.code().value))


if __name__ == '__main__':
    logger.info('Client for email service.')
