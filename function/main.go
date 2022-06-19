package main

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	runtime "github.com/aws/aws-lambda-go/lambda"
)

func handleRequest(ctx context.Context, event events.SQSEvent) (string, error) {
	return "hello", nil
}

func main() {
	runtime.Start(handleRequest)
}