CREATE TABLE namespace (
  id binary(20) NOT NULL COMMENT 'sha1(name)',
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL, /* TODO: Remove. A namespace does not have a namespace. */
  name varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  phase enum('active', 'terminating') COLLATE utf8mb4_unicode_ci NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE namespace_condition (
  namespace_id binary(20) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (namespace_id, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE node (
  id binary(20) NOT NULL,
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
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
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE node_condition (
  node_id binary(20) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  last_heartbeat bigint unsigned NOT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (node_id, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE node_volume (
  node_id binary(20) NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  device_path varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  mounted enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (node_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod (
  id binary(20) NOT NULL COMMENT 'sha1(namespace/name)',
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  node_name varchar(253) NOT NULL,
  nominated_node_name varchar(253) NOT NULL,
  ip varchar(255) NOT NULL,
  restart_policy enum('always', 'on_failure', 'never') COLLATE utf8mb4_unicode_ci NOT NULL,
  cpu_limits bigint unsigned NOT NULL,
  cpu_requests bigint unsigned NOT NULL,
  memory_limits bigint unsigned NOT NULL,
  memory_requests bigint unsigned NOT NULL,
  phase enum('pending', 'running', 'succeeded', 'failed') COLLATE utf8mb4_unicode_ci NOT NULL,
  reason varchar(255) NULL DEFAULT NULL,
  message varchar(255) NULL DEFAULT NULL,
  qos enum('guaranteed', 'burstable', 'best_effort') COLLATE utf8mb4_unicode_ci NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod_condition (
  pod_id binary(20) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  last_probe bigint unsigned NULL DEFAULT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (pod_id, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod_owner (
  pod_id binary(20) NOT NULL,
  kind enum('daemon_set', 'node', 'replica_set', 'stateful_set', 'job') COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  controller enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  block_owner_deletion enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (pod_id, uid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod_pvc (
  pod_id binary(20) NOT NULL,
  volume_name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  claim_name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  read_only enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (pod_id, volume_name, claim_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod_volume (
  pod_id binary(20) NOT NULL,
  volume_name varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  source longtext NOT NULL,
  PRIMARY KEY (pod_id, volume_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE container (
  id binary(20) NOT NULL COMMENT 'sha1(pod.namespace/pod.name/name)',
  pod_id binary(20) NOT NULL,
  name varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  image varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  cpu_limits bigint unsigned NOT NULL,
  cpu_requests bigint unsigned NOT NULL,
  memory_limits bigint unsigned NOT NULL,
  memory_requests bigint unsigned NOT NULL,
  state enum('waiting', 'running', 'terminated') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  state_details longtext NOT NULL,
  ready enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  started enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  restart_count smallint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE container_device (
  container_id binary(20) NOT NULL,
  pod_id binary(20) NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  path varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (container_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE container_mount (
  container_id binary(20) NOT NULL,
  pod_id binary(20) NOT NULL,
  volume_name varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  path varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  sub_path varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  read_only enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (container_id, volume_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE container_log (
  container_id binary(20) NOT NULL,
  pod_id binary(20) NOT NULL,
  logs longtext NOT NULL,
  last_update bigint NOT NULL,

  PRIMARY KEY (container_id, pod_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE deployment (
  id binary(20) NOT NULL,
  namespace varchar(63) COLLATE utf8mb4_unicode_ci  NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  desired_replicas int unsigned NOT NULL,
  strategy enum('recreate', 'rolling_update') COLLATE utf8mb4_unicode_ci NOT NULL,
  min_ready_seconds int unsigned NOT NULL,
  progress_deadline_seconds int unsigned NOT NULL,
  paused enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  actual_replicas int unsigned NOT NULL,
  updated_replicas int unsigned NOT NULL,
  ready_replicas int unsigned NOT NULL,
  available_replicas int unsigned NOT NULL,
  unavailable_replicas int unsigned NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE deployment_condition (
  deployment_id binary(20) NOT NULL,
  type enum('available', 'progressing', 'replica_failure') COLLATE utf8mb4_unicode_ci NOT NULL,
  status varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  last_update bigint unsigned NOT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (deployment_id, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE service (
  id binary(20) NOT NULL,
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  type enum('cluster_ip', 'node_port', 'load_balancer', 'external_name') COLLATE utf8mb4_unicode_ci NOT NULL,
  cluster_ip varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  cluster_ips varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  external_ips varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  session_affinity enum('client_ip', 'none') COLLATE utf8mb4_unicode_ci NOT NULL,
  external_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  external_traffic_policy enum('cluster', 'local') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  health_check_node_port int unsigned NOT NULL,
  publish_not_ready_addresses enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  ip_families enum('IPv4', 'IPv6') COLLATE utf8mb4_general_ci NOT NULL,
  ip_family_policy enum('single_stack', 'prefer_dual_stack', 'require_dual_stack') COLLATE utf8mb4_unicode_ci NOT NULL,
  allocate_load_balancer_node_ports enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  load_balancer_class varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  internal_traffic_policy enum('cluster', 'local') COLLATE utf8mb4_unicode_ci NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE selector (
  id binary(20) NOT NULL,
  name varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  value varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE service_selector (
  service_id binary(20) NOT NULL,
  selector_id binary(20) NOT NULL,
  PRIMARY KEY (service_id, selector_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE service_condition (
  service_id binary(20) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status enum('true', 'false', 'unknown') COLLATE utf8mb4_unicode_ci NOT NULL,
  observed_generation bigint unsigned NULL DEFAULT NULL,
  last_transition bigint unsigned NULL DEFAULT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (service_id, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE service_port (
  service_id binary(20) NOT NULL,
  name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  protocol enum('TCP', 'UDP', 'SCTP') COLLATE utf8mb4_general_ci NOT NULL,
  app_protocol varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  port int unsigned NOT NULL,
  target_port varchar(15) COLLATE utf8mb4_unicode_ci NOT NULL,
  node_port int unsigned NOT NULL,
  PRIMARY KEY (service_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE endpoint_slice (
  id binary(20) NOT NULL,
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  address_type enum('IPv4', 'IPv6', 'FQDN') COLLATE utf8mb4_general_ci NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE endpoint (
  id binary(20) NOT NULL,
  endpoint_slice_id binary(20) NOT NULL,
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
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE endpoint_target_ref (
  endpoint_slice_id binary(20) NOT NULL,
  kind enum('pod', 'node') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  api_version varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (endpoint_slice_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE endpoint_slice_label (
  endpoint_slice_id binary(20) NOT NULL,
  label_id binary(20) NOT NULL,
  PRIMARY KEY (endpoint_slice_id, label_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE ingress (
  id binary(20) NOT NULL,
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE ingress_tls (
  ingress_id binary(20) NOT NULL,
  tls_host varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  tls_secret varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  PRIMARY KEY (ingress_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE ingress_backend_service (
  service_id binary(20) NOT NULL,
  ingress_id binary(20) NOT NULL,
  ingress_rule_id binary(20) NULL DEFAULT NULL,
  service_name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  service_port_name varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  service_port_number int unsigned NULL DEFAULT NULL,
  PRIMARY KEY (service_id, ingress_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE ingress_backend_resource (
  resource_id binary(20) NOT NULL,
  ingress_id binary(20) NOT NULL,
  ingress_rule_id binary(20) NULL DEFAULT NULL,
  api_group varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  kind varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (resource_id, ingress_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE ingress_rule (
  id binary(20) NOT NULL,
  backend_id binary(20) NOT NULL,
  ingress_id binary(20) NOT NULL,
  host varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  path varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  path_type varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE replica_set (
  id binary(20) NOT NULL,
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  desired_replicas int unsigned NOT NULL,
  min_ready_seconds int unsigned NOT NULL,
  actual_replicas int unsigned NOT NULL,
  fully_labeled_replicas int unsigned NOT NULL,
  ready_replicas int unsigned NOT NULL,
  available_replicas int unsigned NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE replica_set_condition (
  replica_set_id binary(20) NOT NULL,
  type enum('replica_failure') COLLATE utf8mb4_unicode_ci NOT NULL,
  status varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (replica_set_id, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE replica_set_owner (
  replica_set_id binary(20) NOT NULL,
  kind enum('deployment') COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  controller enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  block_owner_deletion enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (replica_set_id, uid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE daemon_set (
  id binary(20) NOT NULL,
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  update_strategy enum('rolling_update', 'on_delete') COLLATE utf8mb4_unicode_ci NOT NULL,
  min_ready_seconds int unsigned NOT NULL,
  desired_number_scheduled int unsigned NOT NULL,
  current_number_scheduled int unsigned NOT NULL,
  number_misscheduled int unsigned NOT NULL,
  number_ready int unsigned NOT NULL,
  update_number_scheduled int unsigned NOT NULL,
  number_available int unsigned NOT NULL,
  number_unavailable int unsigned NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE daemon_set_condition (
  daemon_set_id binary(20) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (daemon_set_id, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE stateful_set (
  id binary(20) NOT NULL,
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  desired_replicas int unsigned NOT NULL,
  service_name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  pod_management_policy enum('ordered_ready', 'parallel') COLLATE utf8mb4_unicode_ci NOT NULL,
  update_strategy enum('rolling_update', 'on_delete') COLLATE utf8mb4_unicode_ci NOT NULL,
  min_ready_seconds int unsigned NOT NULL,
  persistent_volume_claim_retention_policy_when_deleted enum('retain', 'delete') COLLATE utf8mb4_unicode_ci NOT NULL,
  persistent_volume_claim_retention_policy_when_scaled enum('retain', 'delete') COLLATE utf8mb4_unicode_ci NOT NULL,
  ordinals int unsigned NOT NULL,
  actual_replicas int unsigned NOT NULL,
  ready_replicas int unsigned NOT NULL,
  current_replicas int unsigned NOT NULL,
  updated_replicas int unsigned NOT NULL,
  available_replicas int unsigned NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE stateful_set_condition (
  stateful_set_id binary(20) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (stateful_set_id, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE secret (
  id binary(20) NOT NULL,
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  immutable enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE config_map (
  id binary(20) NOT NULL,
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  immutable enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE data (
  id binary(20) NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  value mediumblob NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE secret_data (
  secret_id binary(20) NOT NULL,
  data_id binary(20) NOT NULL,
  PRIMARY KEY (secret_id, data_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE config_map_data (
  config_map_id binary(20) NOT NULL,
  data_id binary(20) NOT NULL,
  PRIMARY KEY (config_map_id, data_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE label (
  id binary(20) NOT NULL,
  name varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  value varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod_label (
  pod_id binary(20) NOT NULL,
  label_id binary(20) NOT NULL,
  PRIMARY KEY (pod_id, label_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE replica_set_label (
  replica_set_id binary(20) NOT NULL,
  label_id binary(20) NOT NULL,
  PRIMARY KEY (replica_set_id, label_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE deployment_label (
  deployment_id binary(20) NOT NULL,
  label_id binary(20) NOT NULL,
  PRIMARY KEY (deployment_id, label_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE daemon_set_label (
  daemon_set_id binary(20) NOT NULL,
  label_id binary(20) NOT NULL,
  PRIMARY KEY (daemon_set_id, label_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE stateful_set_label (
  stateful_set_id binary(20) NOT NULL,
  label_id binary(20) NOT NULL,
  PRIMARY KEY (stateful_set_id, label_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pvc_label (
  pvc_id binary(20) NOT NULL,
  label_id binary(20) NOT NULL,
  PRIMARY KEY (pvc_id, label_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE namespace_label (
  namespace_id binary(20) NOT NULL,
  label_id binary(20) NOT NULL,
  PRIMARY KEY (namespace_id, label_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE node_label (
  node_id binary(20) NOT NULL,
  label_id binary(20) NOT NULL,
  PRIMARY KEY (node_id, label_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE secret_label (
  secret_id binary(20) NOT NULL,
  label_id binary(20) NOT NULL,
  PRIMARY KEY (secret_id, label_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE config_map_label (
  config_map_id binary(20) NOT NULL,
  label_id binary(20) NOT NULL,
  PRIMARY KEY (config_map_id, label_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE service_label (
  service_id binary(20) NOT NULL,
  label_id binary(20) NOT NULL,
  PRIMARY KEY (service_id, label_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE event (
  id binary(20) NOT NULL,
  namespace varchar(63) NOT NULL,
  name varchar(270) NOT NULL,
  uid varchar(255) NOT NULL,
  resource_version varchar(255) NOT NULL,
  reporting_controller varchar(253) NOT NULL,
  reporting_instance varchar(253) NOT NULL,
  action varchar(255) NOT NULL,
  reason varchar(255) NOT NULL,
  note text NOT NULL,
  type varchar(255) NOT NULL,
  reference_kind varchar(255) NOT NULL,
  reference_namespace varchar(63) NOT NULL,
  reference_name varchar(253) NOT NULL,
  first_seen bigint unsigned NOT NULL,
  last_seen bigint unsigned NOT NULL,
  count int unsigned NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod_metrics (
  namespace varchar(63) NOT NULL,
  pod_name varchar(253) NOT NULL,
  container_name varchar(63) NOT NULL,
  timestamp bigint unsigned NOT NULL,
  duration bigint unsigned NOT NULL,
  cpu_usage float NOT NULL,
  memory_usage float NOT NULL,
  storage_usage float NOT NULL,
  ephemeral_storage_usage float NOT NULL,
  PRIMARY KEY (namespace, pod_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pvc (
  id binary(20) NOT NULL,
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  desired_access_modes tinyint unsigned NOT NULL,
  actual_access_modes tinyint unsigned NOT NULL,
  minimum_capacity bigint unsigned NULL DEFAULT NULL,
  actual_capacity bigint unsigned NOT NULL,
  phase enum('pending', 'bound', 'lost') COLLATE utf8mb4_unicode_ci NOT NULL,
  volume_name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  volume_mode enum('block', 'filesystem') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  storage_class varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pvc_condition (
  pvc_id binary(20) NOT NULL,
  type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  last_probe bigint unsigned NULL DEFAULT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (pvc_id, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE persistent_volume (
  id binary(20) NOT NULL,
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  capacity bigint unsigned NOT NULL,
  phase enum('pending', 'available', 'bound', 'released', 'failed') COLLATE utf8mb4_unicode_ci NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  access_modes tinyint unsigned NULL DEFAULT NULL,
  volume_mode enum('block', 'filesystem') COLLATE utf8mb4_unicode_ci NOT NULL,
  volume_source_type varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  storage_class varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  volume_source longtext NOT NULL,
  reclaim_policy enum('recycle', 'delete', 'retain') COLLATE utf8mb4_unicode_ci NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE persistent_volume_claim_ref (
  persistent_volume_id binary(20) NOT NULL,
  kind varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (persistent_volume_id, uid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE job (
  id binary(20) NOT NULL,
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  parallelism int unsigned NULL DEFAULT NULL,
  completions int unsigned NULL DEFAULT NULL,
  active_deadline_seconds bigint unsigned NULL DEFAULT NULL,
  backoff_limit int unsigned NULL DEFAULT NULL,
  ttl_seconds_after_finished int unsigned NULL DEFAULT NULL,
  completion_mode enum('non_indexed', 'indexed') COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  suspend enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  start_time bigint unsigned NULL DEFAULT NULL,
  completion_time bigint unsigned NULL DEFAULT NULL,
  active int unsigned NOT NULL,
  succeeded int unsigned NOT NULL,
  failed int unsigned NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE job_condition (
  job_id binary(20) NOT NULL,
  type enum('suspended', 'complete', 'failed', 'failure_target') COLLATE utf8mb4_unicode_ci NOT NULL,
  status enum('true', 'false', 'unknown') COLLATE utf8mb4_unicode_ci NOT NULL,
  last_probe bigint unsigned NULL DEFAULT NULL,
  last_transition bigint unsigned NOT NULL,
  reason varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  message text,
  PRIMARY KEY (job_id, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE job_label (
  job_id binary(20) NOT NULL,
  label_id binary(20) NOT NULL,
  PRIMARY KEY (job_id, label_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE cron_job (
  id binary(20) NOT NULL,
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  schedule varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  timezone varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  starting_deadline_seconds bigint unsigned NOT NULL,
  concurrency_policy enum('allow', 'forbid', 'replace') COLLATE utf8mb4_unicode_ci NOT NULL,
  suspend enum('n', 'y') COLLATE utf8mb4_unicode_ci NOT NULL,
  active int unsigned NOT NULL,
  successful_jobs_history_limit int unsigned NOT NULL,
  failed_jobs_history_limit int unsigned NOT NULL,
  last_schedule_time bigint unsigned NULL DEFAULT NULL,
  last_successful_time bigint unsigned NULL DEFAULT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE cron_job_label (
  cron_job_id binary(20) NOT NULL,
  label_id binary(20) NOT NULL,
  PRIMARY KEY (cron_job_id, label_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE kubernetes_schema (
  id int unsigned NOT NULL AUTO_INCREMENT,
  version varchar(64) NOT NULL,
  timestamp bigint unsigned NOT NULL,
  success enum('n', 'y') DEFAULT NULL,
  reason text DEFAULT NULL,
  PRIMARY KEY (id),
  CONSTRAINT idx_kubernetes_schema_version UNIQUE (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

INSERT INTO kubernetes_schema (version, timestamp, success, reason)
VALUES ('0.1.0', UNIX_TIMESTAMP() * 1000, 'y', 'Initial import');
