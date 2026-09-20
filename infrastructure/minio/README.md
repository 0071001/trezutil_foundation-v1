# Object Storage

Local development uses MinIO as an S3-compatible object store.

Production may use managed S3-compatible storage. Media metadata remains in PostgreSQL; binary data does not.
