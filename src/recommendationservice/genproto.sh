#!/bin/bash -eu

# script to compile python protos

python -m grpc_tools.protoc -I../../pb --python_out=. --grpc_python_out=. ../../pb/demo.proto
