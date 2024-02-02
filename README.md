# Icinga Kubernetes

Icinga Kubernetes is a set of components for monitoring and visualizing Kubernetes resources,
consisting of

* the Icinga Kubernetes daemon, which uses the Kubernetes API to monitor the configuration and
  status changes of Kubernetes resources synchronizing every change in a database, and
* [Icinga Kubernetes Web](https://icinga.com/docs/icinga-kubernetes-web)
  which connects to the database for visualizing Kubernetes resources and their state.

![Icinga Kubernetes Overview](doc/res/icinga-kubernetes-overview.png)

Any of the Icinga Kubernetes components can run either inside or outside Kubernetes clusters,
including the database.
At the moment it is only possible to monitor one Kubernetes cluster per Icinga Kubernetes installation.

![Icinga Kubernetes Web Stateful Set](doc/res/icinga-kubernetes-web-stateful-set.png)
![Icinga Kubernetes Web Service](doc/res/icinga-kubernetes-web-service.png)
![Icinga Kubernetes Web Pod](doc/res/icinga-kubernetes-web-pod.png)

## Documentation

Icinga Kubernetes documentation is available at [icinga.com/docs](https://icinga.com/docs/icinga-kubernetes).

## License

Icinga Kubernetes and the Icinga Kubernetes documentation are licensed under the terms of the
[GNU Affero General Public License Version 3](LICENSE).
