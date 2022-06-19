#!/bin/bash
cd function
GOOS=linux go build main.go
zip function.zip main
cd ..


# aws lambda create-function --function-name go-lambda-eks \
# --zip-file fileb://function/function.zip --handler main --runtime go1.x \
# --role arn:aws:iam::012345678912:role/service-role/lambda-eks-role

aws lambda update-function-code \
    --function-name  go-lambda-eks \
    --zip-file fileb://function/function.zip