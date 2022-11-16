# AWS ECR Prometheus Exporter
A simple Golang service to export metrics about AWS ECR.

## Implementation
This service operates by running a webserver that hosts the metrics to be scraped by
Prometheus. The metrics are updated from the AWS ECR API periodically on a cron schedule.

## Usage
This service uses `viper` for configuration and has very minimal configuration with sane
defaults. Below is a table of all configurable element, their default values, and their
descriptions. All configurable elements can be set via environment variables when prefixed
with `AWS_ECR_EXPORTER`.

| Element | Description | Default | Values |
| --- | --- | --- | --- |
| `AWS_ECR_EXPORTER_CRON_SCHEDULE` | The cron schedule that triggers the updating of the metrics. | `0 0 * * * *` | N/A |
| `AWS_ECR_EXPORTER_LOG_FORMAT` | The output format of the logs of the exporter. | `logfmt` | `json`, `logfmt`, `text` |
| `AWS_ECR_EXPORTER_LOG_LEVEL` | The verbosity level of the logs of the exporter. | `info` | `debug`, `info`, `warn`, `error`, `fatal` |
| `AWS_ECR_EXPORTER_WEB_HOST` | The address/host the webserver will bind against. | `0.0.0.0` | N/A |
| `AWS_ECR_EXPORTER_WEB_PORT` | The port the webserver will bind against. | `9090` | N/A |
| `AWS_ECR_EXPORTER_WEB_METRICS_PATH` | The path the metrics will be exposed on. | `/metrics` | N/A |

## Metrics
This service exposes a few, select metrics for the repositories, images, and scan findings
that exist within the AWS ECR registry.

### Repositories
The table below describes the metrics exported for the repositories in AWS ECR.

| Metric | Type | Description |
| --- | --- | --- |
| `aws_ecr_repository_count` | Gauge | The total number of repositories in the AWS ECR registry. |
| `aws_ecr_repository_info` | Gauge | Informational metric providing context via labels. |

### Images
The table below describes the metrics exported for the image in AWS ECR repositories.

| Metric | Type | Description |
| --- | --- | --- |
| `aws_ecr_image_size_bytes` | Gauge | The size of the AWS ECR image in bytes. |
| `aws_ecr_image_pushed_timestamp_seconds` | Gauge | The timestamp at which the image was pushed to AWS ECR. |

### Image Scan Findings
The table below describes the metrics exported for the image scan findings by AWS ECR.
| Metric | Type | Description |
| --- | --- | --- |
| `aws_ecr_image_scan_findings` | Gauge | The number of findings for an AWS ECR image scan. |
| `aws_ecr_image_scan_completed_timestamp_seconds` | Gauge | The timestamp of the latest completed image scan in AWS ECR. |