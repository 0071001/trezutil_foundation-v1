# Protobuf contracts

This directory contains internal gRPC/Protobuf contracts.

Rules:

- Keep protobuf contracts versioned by package (`v1`, `v2`, ...).
- Do not expose internal protobuf directly as the public REST API.
- Generate language bindings in a build step; generated code should not be hand edited.
- Backward-compatible additions are preferred.
