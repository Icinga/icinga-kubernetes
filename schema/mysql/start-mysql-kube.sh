#!/bin/bash

docker run -p 3306:3306 -v /var/icinga-kubernetes/persistent-database:/var/lib/mysql --name kubedb -itd mysql_pod
