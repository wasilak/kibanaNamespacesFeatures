package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/wasilak/kibanaSpacesFeatures/libs"
)

func main() {
	lambda.Start(libs.HandleRequest)
}
