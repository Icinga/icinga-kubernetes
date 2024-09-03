# Features of Icinga for Kubernetes

## Metric Sync

Icinga for Kubernetes integrates with Prometheus to provide metric synchronization.
This feature allows Icinga to collect and visualize real-time metrics from Kubernetes clusters,
enabling more comprehensive monitoring and alerting capabilities.

### Key Benefits:
- **Improved Monitoring**: Collect detailed metrics from Kubernetes, such as resource utilization and pod health, and display them in Icinga.
- **Enhanced Alerts**: Use the synced metrics to create custom alerts for critical thresholds.
- **Visibility into Cluster Performance**: Gain insights into the performance of your Kubernetes clusters to optimize resource usage and troubleshoot issues faster.

By configuring the Prometheus `url` in the `config.yml`, you can enable this feature to retrieve valuable metrics that enhance the overall monitoring experience in Icinga.