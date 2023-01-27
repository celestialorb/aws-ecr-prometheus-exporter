package main

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
)

var (
	scanFindings = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "aws_ecr_image_scan_findings",
		Help: "The number of findings for an AWS ECR image scan.",
	}, []string{"repository", "digest", "tag", "severity"})

	scanCompleted = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "aws_ecr_image_scan_completed_timestamp_seconds",
		Help: "The timestamp of the latest completed image scan in AWS ECR.",
	}, []string{"repository", "digest", "tag"})
)

// TODO: see if these can be extracted from the Golang AWS SDK (v2).
var severities = []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "INFORMATIONAL"}

func CollectScanMetrics(
	ctx context.Context,
	repository types.Repository,
	image *types.ImageIdentifier,
) {
	logger := logrus.WithFields(logrus.Fields{
		"repository": *repository.RepositoryName,
		"image": map[string]string{
			"digest": *image.ImageDigest,
			"tag":    *image.ImageTag,
		},
	})

	// Extract our AWS ECR client from the given context.
	client := ctx.Value(AwsEcrClientKey{}).(*ecr.Client)

	// Create a paginator to sift through all image scan finding results.
	paginator := ecr.NewDescribeImageScanFindingsPaginator(client, &ecr.DescribeImageScanFindingsInput{
		ImageId:        image,
		RepositoryName: repository.RepositoryName,
	})

	for paginator.HasMorePages() {
		// Rate limit calls to the AWS API.
		rateLimiter.Wait(ctx)

		// Fetch the next page of image scan findings results.
		page, err := paginator.NextPage(ctx)
		if err != nil {
			var snfe *types.ScanNotFoundException
			if errors.As(err, &snfe) {
				logger.WithField("err", err).Debug("scan not found, skipping")
				continue
			}

			logger.WithField("err", err).Warn("failed to retrieve next scan findings page")
			continue
		}

		// If we aren't provided any image scan findings this means a scan has not been
		// performed yet or could not be performed and thus we should skip this image for now.
		if page.ImageScanFindings == nil {
			logger.Info("no image scan findings found, skipping image")
			continue
		}

		logger.Info("setting scan completed timestamp")
		scanCompleted.WithLabelValues(
			*page.RepositoryName,
			*image.ImageDigest,
			*image.ImageTag,
		).Set(float64(page.ImageScanFindings.ImageScanCompletedAt.Unix()))

		// Set scan finding severity metrics.
		logger.Info("setting scan finding severities")
		for _, severity := range severities {
			count := int32(0)
			if findings, ok := page.ImageScanFindings.FindingSeverityCounts[severity]; ok {
				count = findings
			}

			scanFindings.WithLabelValues(
				*page.RepositoryName,
				*image.ImageDigest,
				*image.ImageTag,
				strings.ToLower(severity),
			).Set(float64(count))
			logger.WithFields(logrus.Fields{
				"severity": strings.ToLower(severity),
			}).Debug("set severity findings")
		}
	}
}
