CREATE TABLE pod (
  namespace varchar(63) NOT NULL,
  name varchar(63) NOT NULL,
  uid varchar(63) NOT NULL,
  phase varchar(63) NOT NULL,
  PRIMARY KEY(namespace, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE container_logs (
  namespace varchar(63) NOT NULL,
  pod_name varchar(63) NOT NULL,
  container_name varchar(63) NOT NULL,
  logs mediumblob NOT NULL,
  PRIMARY KEY (namespace, pod_name, container_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE node (
  namespace varchar(63) NOT NULL,
  name varchar(63) NOT NULL,
  pod_cidr varchar(63) NOT NULL,
  unschedulable enum('n', 'y') NOT NULL,
  created bigint unsigned NOT NULL,
  ready enum('n', 'y') NOT NULL,
  PRIMARY KEY (namespace, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE deployment (
  namespace varchar(63) NOT NULL,
  name varchar(63) NOT NULL,
  uid varchar(63) NOT NULL,
  strategy varchar(63) NOT NULL,
  paused tinyint NOT NULL,
  replicas int NOT NULL,
  ready_replicas int NOT NULL,
  available_replicas int NOT NULL,
  unavailable_replicas int NOT NULL,
  collision_count int NOT NULL,
  PRIMARY KEY (namespace, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE replica_set (
   namespace varchar(63) NOT NULL,
   name varchar(63) NOT NULL,
   desired_replicas int NOT NULL,
   actual_replicas int NOT NULL,
   min_ready_seconds int NOT NULL,
   fully_labeled_replicas int NOT NULL,
   ready_replicas int NOT NULL,
   available_replicas int NOT NULL,
   created bigint unsigned NOT NULL,
   PRIMARY KEY (namespace, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE service (
  namespace varchar (63) NOT NULL,
  name varchar (63) NOT NULL,
  uid varchar (63) NOT NULL,
  type varchar (63) NOT NULL,
  cluster_ip varchar (63) NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (namespace, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
