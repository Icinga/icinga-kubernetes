# Icinga for Kubernetes

Icinga for Kubernetes is a set of components for monitoring and visualizing Kubernetes resources,
consisting of

* the Icinga for Kubernetes daemon, which uses the Kubernetes API to monitor the configuration and
  status changes of Kubernetes resources synchronizing every change in a database, and
* [Icinga for Kubernetes Web](https://github.com/Icinga/icinga-kubernetes-web)
  which connects to the database for visualizing Kubernetes resources and their state.

![Icinga for Kubernetes Overview](doc/res/icinga-kubernetes-overview.png)

Though any of the Icinga for Kubernetes components can run either inside or outside Kubernetes clusters,
including the database, common setup approaches include the following:

* All components run inside a Kubernetes cluster.
* All components run outside a Kubernetes cluster.
* Only the Icinga for Kubernetes daemon runs inside a Kubernetes cluster,
  requiring configuration for an external service to connect to the database outside the cluster.

Please **note** that at the moment it is only possible to monitor one Kubernetes cluster per
Icinga for Kubernetes installation.

![Icinga for Kubernetes Web Stateful Set](doc/res/icinga-kubernetes-web-stateful-set.png)
![Icinga for Kubernetes Web Service](doc/res/icinga-kubernetes-web-service.png)
![Icinga for Kubernetes Web Pod](doc/res/icinga-kubernetes-web-pod.png)

## Vision and Roadmap

Although every Kubernetes cluster is different, Icinga for Kubernetes aims to provide a zero-configuration baseline for
monitoring Kubernetes. Our goal is to make it easy to understand the complete state of clusters, including resources,
workloads, relations, and performance. We strive to offer comprehensive monitoring that provides a clear and
intuitive view of clusters' health, helping to identify problems and potential bottlenecks.

The Kubernetes API is leveraged to retrieve information about resources and watch ongoing changes.
This data is stored in a database to reduce pressure on the Kubernetes API and
to enable powerful filtering through a relational model.

Currently, Icinga for Kubernetes utilizes all available information from the Kubernetes API to
determine the state of resources and clusters. In future versions, we plan to integrate metrics.
Upcoming features will also include the use of Icinga Notifications for sending alerts and
supporting multiple clusters.

We welcome your ideas on what should be included in the baseline.
Do not hesitate to share your key metrics, important thresholds,
or correlations used to set up alarms in your environments.

## Optional Features

### Metric Sync

Icinga for Kubernetes integrates with Prometheus to synchronize predefined metrics and display charts in the UI.
In future versions, we plan to incorporate these metrics into state evaluation and alerting.
To enable this feature you just have to configure a Prometheus server URL
that collects metrics from your Kubernetes cluster.

## Documentation

Icinga for Kubernetes documentation is available at [icinga.com/docs](https://icinga.com/docs/icinga-kubernetes).

## License

Icinga for Kubernetes and the Icinga for Kubernetes documentation are licensed under the terms of the
[GNU Affero General Public License Version 3](LICENSE).
