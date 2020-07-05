#!/bin/bash
set -e

# generate go proto files
mkdir -p ./golang
protoc --go_out=plugins=grpc:./golang ./*.proto

# generate html doc
#mkdir -p ../doc
#protoc --doc_out=../doc --doc_opt=html,index.html *.proto