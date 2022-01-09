#!/bin/bash -eu

# protos are loaded dynamically for node, simply copies over the proto.
mkdir -p proto
cp -r ../../pb/* ./proto
