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

UPDATE config
    SET `key` = 'notifications.icingaweb2_url'
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
