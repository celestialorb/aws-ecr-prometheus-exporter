package main

import (
	"context"
	"sort"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	imageSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "aws_ecr_image_size_bytes",
		Help: "The size of the AWS ECR image in bytes.",
	}, []string{"repository", "tag", "digest"})

	imagePushedAt = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "aws_ecr_image_pushed_timestamp_seconds",
		Help: "The timestamp at which the image was pushed to AWS ECR.",
	}, []string{"repository", "tag", "digest"})
)

func CollectImagesMetrics(
	ctx context.Context,
	repository types.Repository,
) {
	logger := log.WithFields(log.Fields{
		"repository": *repository.RepositoryName,
	})

	// Extract our AWS ECR client from the given context.
	client := ctx.Value(AwsEcrClientKey{}).(*ecr.Client)

	// Create our paginator object.
	images := ecr.NewListImagesPaginator(client, &ecr.ListImagesInput{
		RepositoryName: repository.RepositoryName,
	})

	// While we still have pages to sift through, grab the next one and process it.
	for images.HasMorePages() {
		// Rate limit calls to the AWS API.
		err := rateLimiter.Wait(ctx)
		if err != nil {
			logger.Error("failed to wait for rate limiter", err)
		}

		// Fetch the next page of list images results.
		ipage, err := images.NextPage(ctx)
		if err != nil {
			logger.WithFields(log.Fields{
				"err": err,
			}).Fatal("failed to retrieve the next page of images")
		}

		// Skip empty repositories.
		if len(ipage.ImageIds) <= 0 {
			logger.Info("found empty repository, skipping")
			continue
		}

		// Get another paginator for the descriptions of all the images we found.
		logger.Info("describing images in repository")
		descriptions := ecr.NewDescribeImagesPaginator(client, &ecr.DescribeImagesInput{
			RepositoryName: repository.RepositoryName,
			ImageIds:       ipage.ImageIds,
		})

		// Build up an in-memory list of image descriptions so we can sort them as we are unable to sort them via the
		// AWS API call.
		images := []types.ImageDetail{}
		for descriptions.HasMorePages() {
			// Rate limit calls to the AWS API.
			err := rateLimiter.Wait(ctx)
			if err != nil {
				logger.Error("failed to wait for rate limiter", err)
			}

			// Fetch the next page of image descriptions.
			dpage, err := descriptions.NextPage(ctx)
			if err != nil {
				logger.WithFields(log.Fields{
					"err": err,
				}).Fatal("failed to retrieve next page of image descriptions")
			}

			images = append(images, dpage.ImageDetails...)
		}

		// Sort the images by pushed time.
		sort.SliceStable(images, func(ii, jj int) bool {
			return images[ii].ImagePushedAt.After(*images[jj].ImagePushedAt)
		})

		// Grab a copy of our image tag limit.
		tags := viper.GetUint("limits.image.tags.count")
		for _, description := range images {
			for _, tag := range description.ImageTags {
				// Setup a logger instance scoped to this image tag.
				ilogger := logger.WithFields(log.Fields{
					"image": map[string]string{
						"digest": *description.ImageDigest,
						"tag":    tag,
					},
				})

				// Ensure that we're not going over our tag limit.
				if viper.GetBool("limits.image.tags.enabled") && tags <= 0 {
					// If we've hit our limit, log out a warning and continue on without collecting more metrics.
					ilogger.WithFields(log.Fields{
						"reason": "image tag limit reached",
					}).Warn("skipping metric collection for image tag")
					continue
				} else {
					// Decrement our image tag limit count.
					tags -= 1
				}

				ilogger.Info("setting metrics for image")

				// Set the image pushed timestamp metric.
				imagePushedAt.WithLabelValues(
					*description.RepositoryName,
					*description.ImageDigest,
					tag,
				).Set(float64(description.ImagePushedAt.Unix()))
				ilogger.Debug("set image pushed at metric")

				// Set the image size metric.
				imageSize.WithLabelValues(
					*description.RepositoryName,
					*description.ImageDigest,
					tag,
				).Set(float64(*description.ImageSizeInBytes))
				ilogger.Debug("set image size metric")

				go CollectScanMetrics(ctx, repository, &types.ImageIdentifier{
					ImageDigest: description.ImageDigest,
					ImageTag:    &tag,
				})
			}
		}
	}
}
