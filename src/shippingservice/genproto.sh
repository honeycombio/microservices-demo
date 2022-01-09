#!/bin/bash -eu

PATH=$PATH:$GOPATH/bin
protodir=../../pb

protoc --go_out=. --go-grpc_out=require_unimplemented_servers=false:. --proto_path=$protodir $protodir/demo.proto
