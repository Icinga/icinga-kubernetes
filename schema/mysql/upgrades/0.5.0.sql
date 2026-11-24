DROP TABLE IF EXISTS
    resource_annotation,
    resource_label,
    config_map_annotation,
    config_map_label,
    cron_job_annotation,
    cron_job_label,
    daemon_set_annotation,
    daemon_set_label,
    deployment_annotation,
    deployment_label,
    endpoint_slice_label,
    ingress_annotation,
    ingress_label,
    job_annotation,
    job_label,
    namespace_annotation,
    namespace_label,
    node_annotation,
    node_label,
    persistent_volume_annotation,
    persistent_volume_label,
    pod_annotation,
    pod_label,
    pvc_annotation,
    pvc_label,
    replica_set_annotation,
    replica_set_label,
    secret_annotation,
    secret_label,
    service_annotation,
    service_label,
    stateful_set_annotation,
    stateful_set_label,
    selector;

-- The rows of these tables can't be linked to their resources without the
-- dropped link tables. The initial sync on startup fills them again.
DROP TABLE IF EXISTS annotation, label, service_selector;

CREATE TABLE annotation (
  uuid binary(16) NOT NULL,
  resource_uuid binary(16) NOT NULL,
  name varchar(317) COLLATE utf8mb4_unicode_ci NOT NULL,
  value mediumblob NOT NULL,
  PRIMARY KEY (uuid),
  INDEX idx_annotation_resource_uuid (resource_uuid) COMMENT 'Filter for annotations of a resource'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE label (
  uuid binary(16) NOT NULL,
  resource_uuid binary(16) NOT NULL,
  name varchar(317) COLLATE utf8mb4_unicode_ci NOT NULL,
  value varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (uuid),
  INDEX idx_label_resource_uuid (resource_uuid) COMMENT 'Filter for labels of a resource'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE service_selector (
  uuid binary(16) NOT NULL,
  service_uuid binary(16) NOT NULL,
  name varchar(317) COLLATE utf8mb4_unicode_ci NOT NULL,
  value varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  PRIMARY KEY (uuid),
  INDEX idx_service_selector_service_uuid (service_uuid) COMMENT 'Filter for selectors of a service'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

ALTER TABLE service_pod
    ADD INDEX idx_service_pod_pod_uuid (pod_uuid) COMMENT 'Filter for links of a pod';

ALTER TABLE container
    ADD INDEX idx_container_pod_uuid (pod_uuid) COMMENT 'Filter for containers of a pod';

ALTER TABLE init_container
    ADD INDEX idx_init_container_pod_uuid (pod_uuid) COMMENT 'Filter for init containers of a pod';

ALTER TABLE sidecar_container
    ADD INDEX idx_sidecar_container_pod_uuid (pod_uuid) COMMENT 'Filter for sidecar containers of a pod';

ALTER TABLE config
    MODIFY COLUMN `key` enum(
        'notifications.url',
        'notifications.username',
        'notifications.password',
        'notifications.kubernetes_web_url',
        'notifications.icingaweb2_url',
        'prometheus.url',
        'prometheus.insecure',
        'prometheus.username',
        'prometheus.password'
    ) COLLATE utf8mb4_unicode_ci NOT NULL;

-- kubernetes_web_url pointed to the Kubernetes module, while icingaweb2_url is
-- the Icinga Web 2 base URL that event URLs starting with "kubernetes/" are
-- resolved against.
UPDATE config
    SET `key` = 'notifications.icingaweb2_url', value = REGEXP_REPLACE(value, '/kubernetes/?$', '')
    WHERE `key` = 'notifications.kubernetes_web_url';

ALTER TABLE config
    MODIFY COLUMN `key` enum(
        'notifications.url',
        'notifications.username',
        'notifications.password',
        'notifications.icingaweb2_url',
        'prometheus.url',
        'prometheus.insecure',
        'prometheus.username',
        'prometheus.password',
        'prometheus.token',
        'prometheus.token_file'
    ) COLLATE utf8mb4_unicode_ci NOT NULL,
    MODIFY COLUMN value text NOT NULL ;

ALTER TABLE config_map
    ADD COLUMN deleted bigint unsigned NULL DEFAULT NULL,
    ADD INDEX idx_config_map_deleted (cluster_uuid, deleted) COMMENT 'Filter for purging soft-deleted config maps';

ALTER TABLE cron_job
    ADD COLUMN deleted bigint unsigned NULL DEFAULT NULL,
    ADD INDEX idx_cron_job_deleted (cluster_uuid, deleted) COMMENT 'Filter for purging soft-deleted cron jobs';

ALTER TABLE daemon_set
    ADD COLUMN deleted bigint unsigned NULL DEFAULT NULL,
    ADD INDEX idx_daemon_set_deleted (cluster_uuid, deleted) COMMENT 'Filter for purging soft-deleted daemon sets';

ALTER TABLE deployment
    ADD COLUMN deleted bigint unsigned NULL DEFAULT NULL,
    ADD INDEX idx_deployment_deleted (cluster_uuid, deleted) COMMENT 'Filter for purging soft-deleted deployments';

ALTER TABLE endpoint_slice
    ADD COLUMN deleted bigint unsigned NULL DEFAULT NULL,
    ADD INDEX idx_endpoint_slice_deleted (cluster_uuid, deleted) COMMENT 'Filter for purging soft-deleted endpoint slices';

ALTER TABLE ingress
    ADD COLUMN deleted bigint unsigned NULL DEFAULT NULL,
    ADD INDEX idx_ingress_deleted (cluster_uuid, deleted) COMMENT 'Filter for purging soft-deleted ingresses';

ALTER TABLE job
    ADD COLUMN deleted bigint unsigned NULL DEFAULT NULL,
    ADD INDEX idx_job_deleted (cluster_uuid, deleted) COMMENT 'Filter for purging soft-deleted jobs';

ALTER TABLE namespace
    ADD COLUMN deleted bigint unsigned NULL DEFAULT NULL,
    ADD INDEX idx_namespace_deleted (cluster_uuid, deleted) COMMENT 'Filter for purging soft-deleted namespaces';

ALTER TABLE node
    ADD COLUMN deleted bigint unsigned NULL DEFAULT NULL,
    ADD INDEX idx_node_deleted (cluster_uuid, deleted) COMMENT 'Filter for purging soft-deleted nodes';

ALTER TABLE persistent_volume
    ADD COLUMN deleted bigint unsigned NULL DEFAULT NULL,
    ADD INDEX idx_persistent_volume_deleted (cluster_uuid, deleted) COMMENT 'Filter for purging soft-deleted persistent volumes';

ALTER TABLE pod
    ADD COLUMN deleted bigint unsigned NULL DEFAULT NULL,
    ADD INDEX idx_pod_deleted (cluster_uuid, deleted) COMMENT 'Filter for purging soft-deleted pods';

ALTER TABLE pvc
    ADD COLUMN deleted bigint unsigned NULL DEFAULT NULL,
    ADD INDEX idx_pvc_deleted (cluster_uuid, deleted) COMMENT 'Filter for purging soft-deleted PVCs';

ALTER TABLE replica_set
    ADD COLUMN deleted bigint unsigned NULL DEFAULT NULL,
    ADD INDEX idx_replica_set_deleted (cluster_uuid, deleted) COMMENT 'Filter for purging soft-deleted replica sets';

ALTER TABLE secret
    ADD COLUMN deleted bigint unsigned NULL DEFAULT NULL,
    ADD INDEX idx_secret_deleted (cluster_uuid, deleted) COMMENT 'Filter for purging soft-deleted secrets';

ALTER TABLE service
    ADD COLUMN deleted bigint unsigned NULL DEFAULT NULL,
    ADD INDEX idx_service_deleted (cluster_uuid, deleted) COMMENT 'Filter for purging soft-deleted services';

ALTER TABLE stateful_set
    ADD COLUMN deleted bigint unsigned NULL DEFAULT NULL,
    ADD INDEX idx_stateful_set_deleted (cluster_uuid, deleted) COMMENT 'Filter for purging soft-deleted stateful sets';

INSERT INTO kubernetes_schema (version, timestamp, success, reason)
    VALUES ('v0.5.0', UNIX_TIMESTAMP() * 1000, 'y', NULL);
