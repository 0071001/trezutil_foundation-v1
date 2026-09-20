# Trezutil Engineering Guide

## Branching

Use short-lived feature branches. Keep commits focused and reversible.

## Definition of Done

A business change is complete only when:

- contract is updated;
- authorization is defined;
- validation is implemented;
- idempotency is handled when required;
- events are documented/emitted where required;
- tests cover normal and failure paths;
- logs/metrics are adequate;
- sensitive data handling is reviewed;
- documentation is updated.

## Error handling

Use typed/application error codes. Avoid leaking internal provider or database details to end users.

## Money

Use integer minor units and explicit currency. Never float.

## Time

Use UTC internally. Convert for display based on user/organization locale.

## Concurrency

Use optimistic locking/version fields where concurrent updates are possible, especially inventory, listing availability and payment state.

## External integrations

Every external connector should define:

- timeout;
- retry policy;
- idempotency strategy;
- health status;
- error mapping;
- observability;
- secret/config requirements.
