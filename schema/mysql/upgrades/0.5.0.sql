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
        'prometheus.password'
    ) COLLATE utf8mb4_unicode_ci NOT NULL;

INSERT INTO kubernetes_schema (version, timestamp, success, reason)
    VALUES ('v0.5.0', UNIX_TIMESTAMP() * 1000, 'y', NULL);
