#!/usr/bin/env bash
go build -tags netgo -ldflags '-s -w' -o app
