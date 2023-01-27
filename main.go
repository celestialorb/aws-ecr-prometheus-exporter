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

var rateLimiter rate.Limiter

func main() {
	// Setup our configuration.
	viper.SetDefault("log.format", "logfmt")
	viper.SetDefault("log.level", "info")
	viper.SetDefault("web.host", "0.0.0.0")
	viper.SetDefault("web.port", 9090)
	viper.SetDefault("web.metrics.path", "/metrics")
	viper.SetDefault("cron.schedule", "0 0 * * * *")

	// Setup the rate limiter configuration.
	viper.SetDefault("rate.limit.bursts", 1)
	viper.SetDefault("rate.limit.frequency", 2)

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

	// Instantiate our rate limiter instance from our service configuration.
	log.Info("instantiating rate limiter")
	rateLimiter = *rate.NewLimiter(
		rate.Limit(viper.GetFloat64("rate.limit.frequency")),
		viper.GetInt("rate.limit.bursts"),
	)
	log.WithFields(log.Fields{
		"bursts":    rateLimiter.Burst(),
		"frequency": rateLimiter.Limit(),
	}).Info("rate limiter instantiated")

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
