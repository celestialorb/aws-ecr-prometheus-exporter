package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	scanFindings = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "aws_ecr_image_scan_findings",
		Help: "The number of findings for an AWS ECR image scan.",
	})
)