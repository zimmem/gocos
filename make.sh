#!/bin/bash

mkdir -p target

name=gocos
export GOOS=linux GOARCH=amd64 prefix=
go build gocos
zip target/${name}_${GOOS}_${GOARCH}.zip $name$prefix
go clean

export GOOS=windows GOARCH=amd64 prefix=.exe
go build gocos
zip target/${name}_${GOOS}_${GOARCH}.zip $name$prefix
go clean