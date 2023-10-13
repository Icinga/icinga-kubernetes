CREATE TABLE node (
  id binary(20) NOT NULL,
  canonical_name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  name varchar(253) COLLATE utf8mb4_unicode_ci NOT NULL,
  uid varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  resource_version varchar(255) NOT NULL,
  created bigint unsigned NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
