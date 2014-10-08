#!/bin/bash
go-bindata -pkg admin -o admin/bindata.go admin/data
go build
