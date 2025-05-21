CREATE TABLE node (
  id binary(20) NOT NULL,
  canonical_name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE namespace (
  id binary(20) NOT NULL,
  canonical_name varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE pod (
  id binary(20) NOT NULL,
  canonical_name varchar(317) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'namespace/name',
  namespace varchar(63) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE container_log (
  container_id binary(20) NOT NULL,
  pod_id binary(20) NOT NULL,
  time longtext NOT NULL,
  log longtext NOT NULL,
  PRIMARY KEY (container_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
