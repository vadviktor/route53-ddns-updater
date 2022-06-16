#!/usr/bin/env bash

GOOS=linux GOARCH=amd64 go build -a -race -buildvcs=false -o ./build/ddns-amd64
GOOS=linux GOARCH=arm64 GOARM=7 go build -a -buildvcs=false -o ./build/ddns-arm64v7
