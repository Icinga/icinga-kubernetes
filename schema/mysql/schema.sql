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
  created bigint unsigned NOT NULL,
  PRIMARY KEY (namespace, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE replica_set (
   namespace varchar(63) NOT NULL,
   name varchar(63) NOT NULL,
   uid varchar(63) NOT NULL,
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

CREATE TABLE daemon_set (
  namespace varchar(63) NOT NULL,
  name varchar(63) NOT NULL,
  uid varchar(63) NOT NULL,
  min_ready_seconds int NOT NULL,
  current_number_scheduled int NOT NULL,
  number_misscheduled int NOT NULL,
  desired_number_scheduled int NOT NULL,
  number_ready int NOT NULL,
  collision_count int NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (namespace, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE stateful_set (
  namespace varchar(63) NOT NULL,
  name varchar(63) NOT NULL,
  uid varchar(63) NOT NULL,
  replicas int NOT NULL,
  service_name varchar(63) NOT NULL,
  ready_replicas int NOT NULL,
  current_replicas int NOT NULL,
  updated_replicas int NOT NULL,
  available_replicas int NOT NULL,
  current_revision varchar(63) NOT NULL,
  update_revision varchar(63) NOT NULL,
  collision_count int NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (namespace, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE event (
  namespace varchar(63) NOT NULL,
  name varchar(63) NOT NULL,
  uid varchar(63) NOT NULL,
  reporting_controller varchar(63) NOT NULL,
  reporting_instance varchar(63) NOT NULL,
  action varchar(63) NOT NULL,
  reason varchar(63) NOT NULL,
  note varchar(63) NOT NULL,
  type varchar(63) NOT NULL,
  reference_kind varchar(63) NOT NULL,
  reference varchar(63) NOT NULL,
  PRIMARY KEY (namespace, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod_metrics (
  namespace varchar(63) NOT NULL,
  pod_name varchar(63) NOT NULL,
  container_name varchar(63) NOT NULL,
  timestamp bigint unsigned NOT NULL,
  duration bigint unsigned NOT NULL,
  cpu_usage float NOT NULL,
  memory_usage float NOT NULL,
  storage_usage float NOT NULL,
  ephemeral_storage_usage float NOT NULL,
  PRIMARY KEY (namespace, pod_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE volumes (
  namespace varchar(63) NOT NULL,
  pod_name varchar(63) NOT NULL,
  name varchar(63) NOT NULL,
  type varchar(255) NOT NULL,
  volume_source longtext NOT NULL,
  PRIMARY KEY (namespace, pod_name, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod_pvc (
  namespace varchar(63) NOT NULL,
  pod_name varchar(63) NOT NULL,
  claim_name varchar(63) NOT NULL,
  read_only tinyint NOT NULL,
  PRIMARY KEY (namespace, pod_name, claim_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE container_volume_mount (
  namespace varchar(63) NOT NULL,
  pod_name varchar(63) NOT NULL,
  mount_name varchar(63) NOT NULL,
  read_only varchar(63) NOT NULL,
  mount_path varchar(63) NOT NULL,
  sub_path varchar(63) NOT NULL,
  PRIMARY KEY (namespace, pod_name, mount_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
