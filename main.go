package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/procyon-projects/chrono"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/time/rate"
)

type AwsEcrClientKey struct{}

var rl rate.Limiter

func main() {
	// Setup our configuration.
	viper.SetDefault("log.format", "logfmt")
	viper.SetDefault("log.level", "info")
	viper.SetDefault("web.host", "0.0.0.0")
	viper.SetDefault("web.port", 9090)
	viper.SetDefault("web.metrics.path", "/metrics")
	viper.SetDefault("cron.schedule", "0 0 * * * *")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetEnvPrefix("AWS_ECR_EXPORTER")
	viper.AutomaticEnv()

	// Setup our logging framework.
	switch viper.GetString("log.format") {
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
	case "logfmt":
	case "text":
	default:
		log.SetFormatter(&log.TextFormatter{})
	}

	// Set the log level.
	level, err := log.ParseLevel(viper.GetString("log.level"))
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Warn("failed to parse log level, defaulting to INFO")
		level = log.InfoLevel
	}
	log.SetLevel(level)
	log.Debug("logger initialized")

	// / Explanation: interval is a time in seconds, so if interval is 1 and requests
	// also 1 then only 1 operation per second is performed.
	// Variables for rate limiting
	number_of_requests := 2
	// Number of seconds for rate interval
	interval := 1
	// Create the rate limiter
	rl = *rate.NewLimiter(rate.Limit(number_of_requests), interval)

	// Trigger a collection of metrics before we start our webserver.
	CollectRepositoryMetrics(context.Background())

	// Setup our scheduler.
	scheduler := chrono.NewDefaultTaskScheduler()
	_, err = scheduler.ScheduleWithCron(
		CollectRepositoryMetrics,
		viper.GetString("cron.schedule"),
	)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Fatal("failed to start scheduler")
	}

	// Add our Prometheus metrics handler.
	log.Debug("adding Prometheus metrics handler")
	http.Handle(viper.GetString("web.metrics.path"), promhttp.Handler())

	// Start our webserver.
	log.Debug("starting webserver")
	err = http.ListenAndServe(
		fmt.Sprintf(
			"%s:%d",
			viper.GetString("web.host"),
			viper.GetInt32("web.port"),
		),
		nil,
	)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Fatal("webserver failed")
	}
}
