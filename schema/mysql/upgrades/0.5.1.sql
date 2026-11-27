ALTER TABLE kubernetes_instance
    ADD COLUMN notifications_discovered_socket_path text NULL DEFAULT NULL,
    ADD COLUMN kubernetes_service_user varchar(255) NULL DEFAULT NULL;

INSERT INTO kubernetes_schema (version, timestamp, success, reason)
    VALUES ('v0.5.1', UNIX_TIMESTAMP() * 1000, 'y', NULL)
    ON DUPLICATE KEY UPDATE version = kubernetes_schema.version;
