CREATE TABLE pod (
  name varchar(63) NOT NULL,
  namespace varchar(63) NOT NULL,
  uid varchar(63) NOT NULL,
  phase varchar(63) NOT NULL,
  PRIMARY KEY(name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE container_logs (
  name varchar(63) NOT NULL,
  logs ... (binary) NOT NULL,
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
