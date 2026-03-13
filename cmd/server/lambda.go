//go:build lambda

package main

import (
	"net/http"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
)

func startLambda(handler http.Handler) {
	lambda.Start(httpadapter.NewV2(handler).ProxyWithContext)
}
