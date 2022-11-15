package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
)

var (
	imageSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "aws_ecr_image_size_bytes",
		Help: "The size of the AWS ECR image in bytes.",
	}, []string{"repository", "tag", "digest"})
)

func CollectImagesMetrics(
	ctx context.Context,
	repository types.Repository,
) {
	// Extract our AWS ECR client from the given context.
	client := ctx.Value(AwsEcrClientKey{}).(*ecr.Client)

	// Create our paginator object.
	images := ecr.NewListImagesPaginator(client, &ecr.ListImagesInput{
		RepositoryName: repository.RepositoryName,
	})

	// While we still have pages to sift through, do.
	for images.HasMorePages() {
		ipage, err := images.NextPage(ctx)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Fatal("failed to retrieve the next page of images")
		}

		descriptions := ecr.NewDescribeImagesPaginator(client, &ecr.DescribeImagesInput{
			RepositoryName: repository.RepositoryName,
			ImageIds: ipage.ImageIds,
		})

		for descriptions.HasMorePages() {
			dpage, err := descriptions.NextPage(ctx)
			if err != nil {
				log.WithFields(log.Fields{
					"err": err,
				}).Fatal("failed to retrieve next page of image descriptions")
			}

			for _, description := range dpage.ImageDetails {
				for _, tag := range description.ImageTags {
					imageSize.WithLabelValues(
						*description.RepositoryName,
						*description.ImageDigest,
						tag,
					).Set(float64(*description.ImageSizeInBytes))
				}
			}
		}
	}
}