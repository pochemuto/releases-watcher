package main

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

var log = logrus.New()

func main() {
	log.Infof("Run")
	limiter := rate.NewLimiter(60*rate.Every(time.Minute), 1)
	i := 0
	for {
		limiter.Wait(context.TODO())
		log.Infof("%d", i)
		i++
	}
}
