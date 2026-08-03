// Package main is the top-level entry point for the backend service.
// It delegates execution to the api package in cmd/api.
package main

import (
	"github.com/amirhosein/weather-fusion/cmd/api"
)

func main() {
	api.Run()
}
