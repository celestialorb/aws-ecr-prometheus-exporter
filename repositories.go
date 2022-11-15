package main

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	repositoryCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "aws_ecr_repository_count",
		Help: "The total number of repositories in AWS ECR.",
	})

	// repositoryInfo = promauto.NewGauge(prometheus.GaugeOpts{
	// 	Name: "aws_ecr_repository_info",
	// 	Help: "TODO",
	// })
)

func CollectRepositoryMetrics(ctx context.Context) {
	// Reconcile our AWS client configuration.
	log.Debug("loading AWS configuration")
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Fatal("failed to load AWS configuration")
	}

	// Create our AWS client object.
	log.Debug("creating AWS ECR client")
	client := ecr.NewFromConfig(cfg)

	// Keep a running total count of the number of repositories.
	count := 0

	// Use the client to get a list of all the repositories in our registry.
	paginator := ecr.NewDescribeRepositoriesPaginator(client, &ecr.DescribeRepositoriesInput{})
	for paginator.HasMorePages() {
		response, err := paginator.NextPage(ctx)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Fatal("failed to retrieve next page of repositories")
		}

		// Calculate the number of repositories in this response, and add it to the
		// running total.
		repositories := len(response.Repositories)
		count += repositories

		log.WithFields(log.Fields{
			"increment": len(response.Repositories),
			"total": count,
		}).Debug("added to running count")
	}

	// Set our repository count metric.
	repositoryCount.Set(float64(count))
}