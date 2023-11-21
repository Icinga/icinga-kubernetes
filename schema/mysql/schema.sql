CREATE TABLE node
(
    id               BINARY(20)                              NOT NULL,
    canonical_name   VARCHAR(253) COLLATE utf8mb4_unicode_ci NOT NULL,
    name             VARCHAR(253) COLLATE utf8mb4_unicode_ci NOT NULL,
    uid              VARCHAR(255) COLLATE utf8mb4_unicode_ci NOT NULL,
    resource_version VARCHAR(255)                            NOT NULL,
    created          BIGINT UNSIGNED                         NOT NULL,
    PRIMARY KEY (id)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin;

CREATE TABLE namespace
(
    id               BINARY(20)                              NOT NULL,
    canonical_name   VARCHAR(63) COLLATE utf8mb4_unicode_ci  NOT NULL,
    name             VARCHAR(63) COLLATE utf8mb4_unicode_ci  NOT NULL,
    uid              VARCHAR(255) COLLATE utf8mb4_unicode_ci NOT NULL,
    resource_version VARCHAR(255)                            NOT NULL,
    created          BIGINT UNSIGNED                         NOT NULL,
    PRIMARY KEY (id)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin;

CREATE TABLE pod
(
    id               BINARY(20)                              NOT NULL,
    canonical_name   VARCHAR(317) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'namespace/name',
    namespace        VARCHAR(63) COLLATE utf8mb4_unicode_ci  NOT NULL,
    name             VARCHAR(253) COLLATE utf8mb4_unicode_ci NOT NULL,
    uid              VARCHAR(255) COLLATE utf8mb4_unicode_ci NOT NULL,
    resource_version VARCHAR(255)                            NOT NULL,
    created          BIGINT UNSIGNED                         NOT NULL,
    PRIMARY KEY (id)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin;

CREATE TABLE log
(
    id             BINARY(20)                              NOT NULL,
    reference_id   BINARY(20)                              NOT NULL,
    container_name VARCHAR(255) COLLATE utf8mb4_unicode_ci NOT NULL,
    time           LONGTEXT                                NOT NULL,
    log            LONGTEXT                                NOT NULL,
    PRIMARY KEY (id)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin;
