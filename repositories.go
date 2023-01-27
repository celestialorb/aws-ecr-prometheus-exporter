package main

import (
	"context"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	repositoryCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "aws_ecr_repository_count",
		Help: "The total number of repositories in the AWS ECR registry.",
	})

	repositoryInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "aws_ecr_repository_info",
		Help: "Informational metric providing context via labels.",
	}, []string{"name", "registry", "scan_on_push", "tag_mutability", "uri"})
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

	// Create our AWS ECR client object.
	log.Debug("creating AWS ECR client")
	client := ecr.NewFromConfig(cfg)

	// Create a new context with the AWS ECR client object injected into it.
	ecrctx := context.WithValue(ctx, AwsEcrClientKey{}, client)

	// Keep a running total count of the number of repositories.
	count := 0

	// Use the client to get a list of all the repositories in our registry.
	paginator := ecr.NewDescribeRepositoriesPaginator(client, &ecr.DescribeRepositoriesInput{})
	for paginator.HasMorePages() {
		// Rate limit calls to the AWS API.
		rateLimiter.Wait(ctx)

		// Fetch the next page of repository descriptions.
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
			"total":     count,
		}).Debug("added to running count")

		for _, repository := range response.Repositories {
			// Set the repository information metric.
			repositoryInfo.WithLabelValues(
				*repository.RepositoryName,
				*repository.RegistryId,
				strconv.FormatBool(repository.ImageScanningConfiguration.ScanOnPush),
				string(repository.ImageTagMutability),
				*repository.RepositoryUri,
			).Set(1)

			// Collect image metrics for the repository.
			go CollectImagesMetrics(ecrctx, repository)
		}
	}

	// Set our repository count metric.
	repositoryCount.Set(float64(count))
}
