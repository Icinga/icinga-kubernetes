CREATE TABLE cluster (
  uuid binary(16) NOT NULL,
  name varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE annotation (
  uuid binary(16) NOT NULL,
  name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  value mediumblob NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE resource_annotation (
  resource_uuid binary(16) NOT NULL,
  annotation_uuid binary(16) NOT NULL,
  PRIMARY KEY (resource_uuid, annotation_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE label (
  uuid binary(16) NOT NULL,
  name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  value varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE resource_label (
  resource_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (resource_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE config_map (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  immutable enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE config_map_annotation (
  config_map_uuid binary(16) NOT NULL,
  annotation_uuid binary(16) NOT NULL,
  PRIMARY KEY (config_map_uuid, annotation_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE config_map_label (
  config_map_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (config_map_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE container (
  uuid binary(16) NOT NULL,
  pod_uuid binary(16) NOT NULL,
  name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  image varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  image_pull_policy enum('Always', 'Never', 'IfNotPresent') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  cpu_limits bigint unsigned NULL DEFAULT NULL,
  cpu_requests bigint unsigned NULL DEFAULT NULL,
  memory_limits bigint unsigned NULL DEFAULT NULL,
  memory_requests bigint unsigned NULL DEFAULT NULL,
  state enum('Waiting', 'Running', 'Terminated') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  state_details longtext NULL DEFAULT NULL,
  ready enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  started enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  restart_count int unsigned NOT NULL,
  icinga_state enum('unknown', 'pending', 'ok', 'warning', 'critical') COLLATE utf8mb4_unicode_ci NOT NULL,
  icinga_state_reason text NULL DEFAULT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE init_container (
  uuid binary(16) NOT NULL,
  pod_uuid binary(16) NOT NULL,
  name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  image varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  image_pull_policy enum('Always', 'Never', 'IfNotPresent') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  cpu_limits bigint unsigned NULL DEFAULT NULL,
  cpu_requests bigint unsigned NULL DEFAULT NULL,
  memory_limits bigint unsigned NULL DEFAULT NULL,
  memory_requests bigint unsigned NULL DEFAULT NULL,
  state enum('Waiting', 'Running', 'Terminated') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  state_details longtext NULL DEFAULT NULL,
  icinga_state enum('unknown', 'pending', 'ok', 'warning', 'critical') COLLATE utf8mb4_unicode_ci NOT NULL,
  icinga_state_reason text NULL DEFAULT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE sidecar_container (
  uuid binary(16) NOT NULL,
  pod_uuid binary(16) NOT NULL,
  name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  image varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  image_pull_policy enum('Always', 'Never', 'IfNotPresent') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  cpu_limits bigint unsigned NULL DEFAULT NULL,
  cpu_requests bigint unsigned NULL DEFAULT NULL,
  memory_limits bigint unsigned NULL DEFAULT NULL,
  memory_requests bigint unsigned NULL DEFAULT NULL,
  state enum('Waiting', 'Running', 'Terminated') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  state_details longtext NULL DEFAULT NULL,
  ready enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  started enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  restart_count int unsigned NOT NULL,
  icinga_state enum('unknown', 'pending', 'ok', 'warning', 'critical') COLLATE utf8mb4_unicode_ci NOT NULL,
  icinga_state_reason text NULL DEFAULT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE container_device (
  container_uuid binary(16) NOT NULL,
  pod_uuid binary(16) NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  path varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (container_uuid, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE container_log (
  container_uuid binary(16) NOT NULL,
  pod_uuid binary(16) NOT NULL,
  logs text NOT NULL,
  last_update bigint NOT NULL,
  PRIMARY KEY (container_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE container_mount (
  container_uuid binary(16) NOT NULL,
  pod_uuid binary(16) NOT NULL,
  volume_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  path varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  sub_path varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  read_only enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (container_uuid, volume_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE cron_job (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  schedule varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  timezone varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  starting_deadline_seconds bigint unsigned NULL DEFAULT NULL,
  concurrency_policy enum('Allow', 'Forbid', 'Replace') COLLATE utf8mb4_unicode_ci NOT NULL,
  suspend enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  successful_jobs_history_limit int unsigned NOT NULL,
  failed_jobs_history_limit int unsigned NOT NULL,
  active int unsigned NOT NULL,
  last_schedule_time bigint unsigned NULL DEFAULT NULL,
  last_successful_time bigint unsigned NULL DEFAULT NULL,
  yaml mediumblob DEFAULT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE cron_job_annotation (
  cron_job_uuid binary(16) NOT NULL,
  annotation_uuid binary(16) NOT NULL,
  PRIMARY KEY (cron_job_uuid, annotation_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE cron_job_label (
  cron_job_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (cron_job_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE daemon_set (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  update_strategy enum('RollingUpdate', 'OnDelete') COLLATE utf8mb4_unicode_ci NOT NULL,
  min_ready_seconds int unsigned NOT NULL,
  desired_number_scheduled int unsigned NOT NULL,
  current_number_scheduled int unsigned NOT NULL,
  number_misscheduled int unsigned NOT NULL,
  number_ready int unsigned NOT NULL,
  update_number_scheduled int unsigned NOT NULL,
  number_available int unsigned NOT NULL,
  number_unavailable int unsigned NOT NULL,
  yaml mediumblob DEFAULT NULL,
  icinga_state enum('unknown', 'ok', 'warning', 'critical') COLLATE utf8mb4_unicode_ci NOT NULL,
  icinga_state_reason text NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE daemon_set_annotation (
  daemon_set_uuid binary(16) NOT NULL,
  annotation_uuid binary(16) NOT NULL,
  PRIMARY KEY (daemon_set_uuid, annotation_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE daemon_set_condition (
  daemon_set_uuid binary(16) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status enum('true', 'false', 'unknown') COLLATE utf8mb4_unicode_ci NOT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (daemon_set_uuid, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE daemon_set_label (
  daemon_set_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (daemon_set_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE daemon_set_owner (
  daemon_set_uuid binary(16) NOT NULL,
  owner_uuid binary(16) NOT NULL,
  kind varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  controller enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  block_owner_deletion enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (daemon_set_uuid, owner_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE deployment (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci  NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  strategy enum('Recreate', 'RollingUpdate') COLLATE utf8mb4_unicode_ci NOT NULL,
  min_ready_seconds int unsigned NOT NULL,
  progress_deadline_seconds int unsigned NOT NULL,
  paused enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  desired_replicas int unsigned NOT NULL,
  actual_replicas int unsigned NOT NULL,
  updated_replicas int unsigned NOT NULL,
  ready_replicas int unsigned NOT NULL,
  available_replicas int unsigned NOT NULL,
  unavailable_replicas int unsigned NOT NULL,
  yaml mediumblob DEFAULT NULL,
  icinga_state enum('unknown', 'ok', 'warning', 'critical') COLLATE utf8mb4_unicode_ci NOT NULL,
  icinga_state_reason text NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE deployment_annotation (
  deployment_uuid binary(16) NOT NULL,
  annotation_uuid binary(16) NOT NULL,
  PRIMARY KEY (deployment_uuid, annotation_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE deployment_condition (
  deployment_uuid binary(16) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status enum('true', 'false', 'unknown') COLLATE utf8mb4_unicode_ci NOT NULL,
  last_update bigint unsigned NOT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (deployment_uuid, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE deployment_label (
  deployment_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (deployment_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE deployment_owner (
  deployment_uuid binary(16) NOT NULL,
  owner_uuid binary(16) NOT NULL,
  kind varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  controller enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  block_owner_deletion enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (deployment_uuid, owner_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE endpoint (
  uuid binary(16) NOT NULL,
  endpoint_slice_uuid binary(16) NOT NULL,
  host_name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  node_name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  ready enum('n', 'y') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  serving enum('n', 'y') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  terminating enum('n', 'y') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  address varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  protocol enum('TCP', 'UDP', 'SCTP') COLLATE utf8mb4_general_ci NOT NULL,
  port int unsigned NOT NULL,
  port_name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  app_protocol varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE endpoint_slice (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  address_type enum('IPv4', 'IPv6', 'FQDN') COLLATE utf8mb4_general_ci NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE endpoint_slice_label (
  endpoint_slice_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (endpoint_slice_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE endpoint_target_ref (
  endpoint_slice_uuid binary(16) NOT NULL,
  kind enum('pod', 'node') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  api_version varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (endpoint_slice_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE event (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  reference_uuid binary(16) NOT NULL,
  namespace varchar(255) NOT NULL,
  name varchar(270) NOT NULL,
  uid varchar(255) NOT NULL,
  resource_version varchar(255) NOT NULL,
  reporting_controller varchar(253) NULL DEFAULT NULL,
  reporting_instance varchar(128) NULL DEFAULT NULL,
  action varchar(128) NULL DEFAULT NULL,
  reason varchar(128) NOT NULL,
  note text NOT NULL,
  type varchar(255) NOT NULL,
  reference_kind varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  reference_namespace varchar(255) NULL DEFAULT NULL,
  reference_name varchar(253) NOT NULL,
  first_seen bigint unsigned NOT NULL,
  last_seen bigint unsigned NOT NULL,
  count int unsigned NOT NULL,
  yaml mediumblob DEFAULT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid),
  INDEX idx_event_created (created) COMMENT 'Filter for deleting old events'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;


CREATE TABLE ingress (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  yaml mediumblob DEFAULT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE ingress_annotation (
  ingress_uuid binary(16) NOT NULL,
  annotation_uuid binary(16) NOT NULL,
  PRIMARY KEY (ingress_uuid, annotation_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE ingress_backend_resource (
  resource_uuid binary(16) NOT NULL,
  ingress_uuid binary(16) NOT NULL,
  ingress_rule_uuid binary(16) NULL DEFAULT NULL,
  api_group varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  kind varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (resource_uuid, ingress_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE ingress_backend_service (
  service_uuid binary(16) NOT NULL,
  ingress_uuid binary(16) NOT NULL,
  ingress_rule_uuid binary(16) NULL DEFAULT NULL,
  service_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  service_port_name varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  service_port_number int unsigned NULL DEFAULT NULL,
  PRIMARY KEY (service_uuid, ingress_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE ingress_label (
  ingress_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (ingress_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE ingress_rule (
  uuid binary(16) NOT NULL,
  backend_uuid binary(16) NOT NULL,
  ingress_uuid binary(16) NOT NULL,
  host varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  path varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  path_type enum('Exact', 'Prefix', 'ImplementationSpecific') COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE ingress_tls (
  ingress_uuid binary(16) NOT NULL,
  tls_host varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  tls_secret varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  PRIMARY KEY (ingress_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE job (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  parallelism int unsigned NULL DEFAULT NULL,
  completions int unsigned NULL DEFAULT NULL,
  active_deadline_seconds bigint unsigned NULL DEFAULT NULL,
  backoff_limit int unsigned NULL DEFAULT NULL,
  ttl_seconds_after_finished int unsigned NULL DEFAULT NULL,
  completion_mode enum('NonIndexed', 'Indexed') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  suspend enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  start_time bigint unsigned NULL DEFAULT NULL,
  completion_time bigint unsigned NULL DEFAULT NULL,
  active int unsigned NOT NULL,
  succeeded int unsigned NOT NULL,
  failed int unsigned NOT NULL,
  yaml mediumblob DEFAULT NULL,
  icinga_state enum('pending', 'ok', 'warning', 'critical', 'unknown') COLLATE utf8mb4_unicode_ci NOT NULL,
  icinga_state_reason text NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE job_annotation (
  job_uuid binary(16) NOT NULL,
  annotation_uuid binary(16) NOT NULL,
  PRIMARY KEY (job_uuid, annotation_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE job_condition (
  job_uuid binary(16) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status enum('true', 'false', 'unknown') COLLATE utf8mb4_unicode_ci NOT NULL,
  last_probe bigint unsigned NULL DEFAULT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (job_uuid, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE job_label (
  job_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (job_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE job_owner (
  job_uuid binary(16) NOT NULL,
  owner_uuid binary(16) NOT NULL,
  kind varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  controller enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  block_owner_deletion enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (job_uuid, owner_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE namespace (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL, /* TODO: Remove. A namespace does not have a namespace. */
  name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  phase enum('Active', 'Terminating') COLLATE utf8mb4_unicode_ci NOT NULL,
  yaml mediumblob DEFAULT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE namespace_annotation (
  namespace_uuid binary(16) NOT NULL,
  annotation_uuid binary(16) NOT NULL,
  PRIMARY KEY (namespace_uuid, annotation_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE namespace_condition (
  namespace_uuid binary(16) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status enum('true', 'false', 'unknown') COLLATE utf8mb4_unicode_ci NOT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (namespace_uuid, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE namespace_label (
  namespace_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (namespace_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE node (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  pod_cidr varchar(255) NOT NULL,
  num_ips int unsigned NOT NULL,
  unschedulable enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  ready enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  cpu_capacity bigint unsigned NOT NULL,
  cpu_allocatable bigint unsigned NOT NULL,
  memory_capacity bigint unsigned NOT NULL,
  memory_allocatable bigint unsigned NOT NULL,
  pod_capacity int unsigned NOT NULL,
  yaml mediumblob DEFAULT NULL,
  roles varchar(255) NOT NULL,
  machine_id varchar(255) NOT NULL,
  system_uuid varchar(255) NOT NULL,
  boot_id varchar(255) NOT NULL,
  kernel_version varchar(255) NOT NULL,
  os_image varchar(255) NOT NULL,
  operating_system varchar(255) NOT NULL,
  architecture varchar(255) NOT NULL,
  container_runtime_version varchar(255) NOT NULL,
  kubelet_version varchar(255) NOT NULL,
  kube_proxy_version varchar(255) NOT NULL,
  icinga_state enum('unknown', 'ok', 'warning', 'critical') COLLATE utf8mb4_unicode_ci NOT NULL,
  icinga_state_reason text NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE node_annotation (
  node_uuid binary(16) NOT NULL,
  annotation_uuid binary(16) NOT NULL,
  PRIMARY KEY (node_uuid, annotation_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE node_condition (
  node_uuid binary(16) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status enum('true', 'false', 'unknown') COLLATE utf8mb4_unicode_ci NOT NULL,
  last_heartbeat bigint unsigned NOT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (node_uuid, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE node_label (
  node_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (node_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE node_volume (
  node_uuid binary(16) NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  device_path varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  mounted enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (node_uuid, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE persistent_volume (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  capacity bigint unsigned NOT NULL,
  phase enum('Pending', 'Available', 'Bound', 'Released', 'Failed') COLLATE utf8mb4_unicode_ci NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  message text NULL DEFAULT NULL,
  access_modes tinyint unsigned NULL DEFAULT NULL,
  volume_mode enum('Filesystem', 'Block') COLLATE utf8mb4_unicode_ci NOT NULL,
  volume_source_type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  storage_class varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  volume_source longtext NOT NULL,
  reclaim_policy enum('Recycle', 'Delete', 'Retain') COLLATE utf8mb4_unicode_ci NOT NULL,
  yaml mediumblob DEFAULT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE persistent_volume_annotation (
  persistent_volume_uuid binary(16) NOT NULL,
  annotation_uuid binary(16) NOT NULL,
  PRIMARY KEY (persistent_volume_uuid, annotation_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE persistent_volume_claim_ref (
  persistent_volume_uuid binary(16) NOT NULL,
  kind varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (persistent_volume_uuid, uid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE persistent_volume_label (
  persistent_volume_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (persistent_volume_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  node_name varchar(253) NULL DEFAULT NULL,
  nominated_node_name varchar(253) NULL DEFAULT NULL,
  ip varchar(255) NULL DEFAULT NULL,
  restart_policy enum('Always', 'OnFailure', 'Never') COLLATE utf8mb4_unicode_ci NOT NULL,
  cpu_limits bigint unsigned NULL DEFAULT NULL,
  cpu_requests bigint unsigned NULL DEFAULT NULL,
  memory_limits bigint unsigned NULL DEFAULT NULL,
  memory_requests bigint unsigned NULL DEFAULT NULL,
  phase enum('Pending', 'Running', 'Succeeded', 'Failed') COLLATE utf8mb4_unicode_ci NOT NULL,
  icinga_state enum('pending', 'ok', 'warning', 'critical', 'unknown') COLLATE utf8mb4_unicode_ci NOT NULL,
  icinga_state_reason text NULL DEFAULT NULL,
  reason varchar(255) NULL DEFAULT NULL,
  message text NULL DEFAULT NULL,
  qos enum('Guaranteed', 'Burstable', 'BestEffort') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  yaml mediumblob DEFAULT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod_annotation (
  pod_uuid binary(16) NOT NULL,
  annotation_uuid binary(16) NOT NULL,
  PRIMARY KEY (pod_uuid, annotation_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod_condition (
  pod_uuid binary(16) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status enum('true', 'false', 'unknown') COLLATE utf8mb4_unicode_ci NOT NULL,
  last_probe bigint unsigned NULL DEFAULT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (pod_uuid, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod_label (
  pod_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (pod_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod_metrics (
  namespace varchar(255) NOT NULL,
  pod_name varchar(253) NOT NULL,
  container_name varchar(255) NOT NULL,
  timestamp bigint unsigned NOT NULL,
  duration bigint unsigned NOT NULL,
  cpu_usage float NOT NULL,
  memory_usage float NOT NULL,
  storage_usage float NOT NULL,
  ephemeral_storage_usage float NOT NULL,
  PRIMARY KEY (namespace, pod_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod_owner (
  pod_uuid binary(16) NOT NULL,
  owner_uuid binary(16) NOT NULL,
  kind varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  controller enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  block_owner_deletion enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (pod_uuid, owner_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod_pvc (
  pod_uuid binary(16) NOT NULL,
  volume_name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  claim_name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  read_only enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (pod_uuid, volume_name, claim_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod_volume (
  pod_uuid binary(16) NOT NULL,
  volume_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  source longtext NOT NULL,
  PRIMARY KEY (pod_uuid, volume_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE prometheus_cluster_metric (
  cluster_uuid binary(16) NOT NULL,
  timestamp bigint NOT NULL,
  category varchar(255) NOT NULL,
  name varchar(255) NOT NULL,
  value double NOT NULL,
  PRIMARY KEY (cluster_uuid, timestamp, category, name),
  INDEX idx_prometheus_cluster_metric_timestamp (timestamp) COMMENT 'Filter for deleting old cluster metrics'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE prometheus_container_metric (
  container_uuid binary(16) NOT NULL,
  timestamp bigint NOT NULL,
  category varchar(255) NOT NULL,
  name varchar(255) NOT NULL,
  value double NOT NULL,
  PRIMARY KEY (container_uuid, timestamp, category, name),
  INDEX idx_prometheus_container_metric_timestamp (timestamp) COMMENT 'Filter for deleting old container metrics'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE prometheus_node_metric (
  node_uuid binary(16) NOT NULL,
  timestamp bigint NOT NULL,
  category varchar(255) NOT NULL,
  name varchar(255) NOT NULL,
  value double NOT NULL,
  PRIMARY KEY (node_uuid, timestamp, category, name),
  INDEX idx_prometheus_node_metric_timestamp (timestamp) COMMENT 'Filter for deleting old node metrics'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE prometheus_pod_metric (
  pod_uuid binary(16) NOT NULL,
  timestamp bigint NOT NULL,
  category varchar(255) NOT NULL,
  name varchar(255) NOT NULL,
  value double NOT NULL,
  PRIMARY KEY (pod_uuid, timestamp, category, name),
  INDEX idx_prometheus_pod_metric_timestamp (timestamp) COMMENT 'Filter for deleting old pod metrics'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pvc (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  desired_access_modes tinyint unsigned NOT NULL,
  actual_access_modes tinyint unsigned NULL DEFAULT NULL,
  minimum_capacity bigint unsigned NULL DEFAULT NULL,
  actual_capacity bigint unsigned NULL DEFAULT NULL,
  phase enum('Pending', 'Bound', 'Lost') COLLATE utf8mb4_unicode_ci NOT NULL,
  volume_name varchar(253) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  volume_mode enum('Block', 'Filesystem') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  storage_class varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  yaml mediumblob DEFAULT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pvc_annotation (
  pvc_uuid binary(16) NOT NULL,
  annotation_uuid binary(16) NOT NULL,
  PRIMARY KEY (pvc_uuid, annotation_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pvc_condition (
  pvc_uuid binary(16) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status enum('true', 'false', 'unknown') COLLATE utf8mb4_unicode_ci NOT NULL,
  last_probe bigint unsigned NULL DEFAULT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (pvc_uuid, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pvc_label (
  pvc_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (pvc_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE replica_set (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  desired_replicas int unsigned NOT NULL,
  min_ready_seconds int unsigned NOT NULL,
  actual_replicas int unsigned NOT NULL,
  fully_labeled_replicas int unsigned NOT NULL,
  ready_replicas int unsigned NOT NULL,
  available_replicas int unsigned NOT NULL,
  yaml mediumblob DEFAULT NULL,
  icinga_state enum('unknown', 'ok', 'warning', 'critical') COLLATE utf8mb4_unicode_ci NOT NULL,
  icinga_state_reason text NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE replica_set_annotation (
  replica_set_uuid binary(16) NOT NULL,
  annotation_uuid binary(16) NOT NULL,
  PRIMARY KEY (replica_set_uuid, annotation_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE replica_set_condition (
  replica_set_uuid binary(16) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status enum('true', 'false', 'unknown') COLLATE utf8mb4_unicode_ci NOT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (replica_set_uuid, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE replica_set_label (
  replica_set_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (replica_set_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE replica_set_owner (
  replica_set_uuid binary(16) NOT NULL,
  owner_uuid binary(16) NOT NULL,
  kind varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  controller enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  block_owner_deletion enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (replica_set_uuid, owner_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE secret (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  immutable enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE secret_annotation (
  secret_uuid binary(16) NOT NULL,
  annotation_uuid binary(16) NOT NULL,
  PRIMARY KEY (secret_uuid, annotation_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE secret_label (
  secret_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (secret_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE selector (
  uuid binary(16) NOT NULL,
  name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  value varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE service (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  cluster_ip varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  cluster_ips varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  type enum('ClusterIP', 'NodePort', 'LoadBalancer', 'ExternalName') COLLATE utf8mb4_unicode_ci NOT NULL,
  external_ips varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  session_affinity enum('None', 'ClientIP') COLLATE utf8mb4_unicode_ci NOT NULL,
  external_name varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  external_traffic_policy enum('Cluster', 'Local') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  health_check_node_port int unsigned NULL DEFAULT NULL,
  publish_not_ready_addresses enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  ip_families enum('IPv4', 'IPv6', 'DualStack', 'Unknown') COLLATE utf8mb4_general_ci NULL DEFAULT NULL,
  ip_family_policy enum('SingleStack', 'PreferDualStack', 'RequireDualStack') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  allocate_load_balancer_node_ports enum('y', 'n') COLLATE utf8mb4_unicode_ci NOT NULL,
  load_balancer_class varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  internal_traffic_policy enum('Cluster', 'Local') COLLATE utf8mb4_unicode_ci NOT NULL,
  yaml mediumblob DEFAULT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE service_annotation (
  service_uuid binary(16) NOT NULL,
  annotation_uuid binary(16) NOT NULL,
  PRIMARY KEY (service_uuid, annotation_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE service_condition (
  service_uuid binary(16) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status enum('true', 'false', 'unknown') COLLATE utf8mb4_unicode_ci NOT NULL,
  observed_generation bigint unsigned NULL DEFAULT NULL,
  last_transition bigint unsigned NULL DEFAULT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (service_uuid, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE service_label (
  service_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (service_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE service_pod (
  service_uuid binary(16) NOT NULL,
  pod_uuid binary(16) NOT NULL,
  PRIMARY KEY (service_uuid, pod_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE service_port (
  service_uuid binary(16) NOT NULL,
  name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  protocol enum('TCP', 'UDP', 'SCTP') COLLATE utf8mb4_general_ci NOT NULL,
  app_protocol varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  port int unsigned NOT NULL,
  target_port varchar(15) COLLATE utf8mb4_unicode_ci NOT NULL,
  node_port int unsigned NOT NULL,
  PRIMARY KEY (service_uuid, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE service_selector (
  service_uuid binary(16) NOT NULL,
  selector_uuid binary(16) NOT NULL,
  PRIMARY KEY (service_uuid, selector_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE stateful_set (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  namespace varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  desired_replicas int unsigned NOT NULL,
  service_name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  pod_management_policy enum('OrderedReady', 'Parallel') COLLATE utf8mb4_unicode_ci NOT NULL,
  update_strategy enum('RollingUpdate', 'OnDelete') COLLATE utf8mb4_unicode_ci NOT NULL,
  min_ready_seconds int unsigned NOT NULL,
  persistent_volume_claim_retention_policy_when_deleted enum('Retain', 'Delete') COLLATE utf8mb4_unicode_ci NOT NULL,
  persistent_volume_claim_retention_policy_when_scaled enum('Retain', 'Delete') COLLATE utf8mb4_unicode_ci NOT NULL,
  ordinals int unsigned NOT NULL,
  actual_replicas int unsigned NOT NULL,
  ready_replicas int unsigned NOT NULL,
  current_replicas int unsigned NOT NULL,
  updated_replicas int unsigned NOT NULL,
  available_replicas int unsigned NOT NULL,
  yaml mediumblob DEFAULT NULL,
  icinga_state enum('unknown', 'ok', 'warning', 'critical') COLLATE utf8mb4_unicode_ci NOT NULL,
  icinga_state_reason text NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE stateful_set_annotation (
  stateful_set_uuid binary(16) NOT NULL,
  annotation_uuid binary(16) NOT NULL,
  PRIMARY KEY (stateful_set_uuid, annotation_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE stateful_set_condition (
  stateful_set_uuid binary(16) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status enum('true', 'false', 'unknown') COLLATE utf8mb4_unicode_ci NOT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (stateful_set_uuid, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE stateful_set_label (
  stateful_set_uuid binary(16) NOT NULL,
  label_uuid binary(16) NOT NULL,
  PRIMARY KEY (stateful_set_uuid, label_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE stateful_set_owner (
  stateful_set_uuid binary(16) NOT NULL,
  owner_uuid binary(16) NOT NULL,
  kind varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  controller enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  block_owner_deletion enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (stateful_set_uuid, owner_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE favorite (
  resource_uuid binary(16) NOT NULL,
  kind varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  username varchar(254) COLLATE utf8mb4_unicode_ci NOT NULL,
  priority int unsigned,
  PRIMARY KEY (resource_uuid, username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE INDEX idx_favorite_username ON favorite(username, kind) COMMENT 'Favorites filtered by username and kind';

CREATE TABLE kubernetes_instance (
  uuid binary(16) NOT NULL,
  cluster_uuid binary(16) NOT NULL,
  version varchar(255) NOT NULL,
  kubernetes_version varchar(255) NOT NULL,
  kubernetes_heartbeat bigint unsigned NULL DEFAULT NULL,
  kubernetes_api_reachable enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  message text NULL DEFAULT NULL,
  heartbeat bigint unsigned NOT NULL,
  PRIMARY KEY (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC;

CREATE TABLE config (
  cluster_uuid binary(16) NOT NULL,
  `key` enum(
    'notifications.url',
    'notifications.username',
    'notifications.password',
    'notifications.kubernetes_web_url',
    'prometheus.url',
    'prometheus.username',
    'prometheus.password'
    ) COLLATE utf8mb4_unicode_ci NOT NULL,
  value varchar(255) NOT NULL,
  locked enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,

  PRIMARY KEY (`key`, cluster_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE kubernetes_schema (
  id int unsigned NOT NULL AUTO_INCREMENT,
  version varchar(255) NOT NULL,
  timestamp bigint unsigned NOT NULL,
  success enum('n', 'y') DEFAULT NULL,
  reason text DEFAULT NULL,
  PRIMARY KEY (id),
  CONSTRAINT idx_kubernetes_schema_version UNIQUE (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

INSERT INTO kubernetes_schema (version, timestamp, success, reason)
VALUES ('0.2.0', UNIX_TIMESTAMP() * 1000, 'y', 'Initial import');
