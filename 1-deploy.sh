#!/bin/bash
cd function
GOOS=linux go build main.go
zip function.zip main
cd ..