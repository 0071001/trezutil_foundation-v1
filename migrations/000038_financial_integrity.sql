-- Financial integrity (remediation phase 1).
--
-- Webhook deliveries become reclaimable: an event whose processing failed, or whose processor stopped midway, can be
-- claimed again; one that contradicts the recorded state is kept for review. Nothing existing is changed or deleted.
--
-- The ledger gets a guard against posting the same line twice (payment or refund, account, direction). If existing
-- data already holds a duplicate, the index is not created and a notice says so: the reconciliation report lists the
-- duplicates, which a person must correct before the index can be added.

-- +goose Up
ALTER TABLE payment_webhook_events ADD COLUMN IF NOT EXISTS claimed_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE payment_webhook_events ADD COLUMN IF NOT EXISTS attempts INTEGER NOT NULL DEFAULT 0;

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM payment_ledger WHERE status = 'posted'
        GROUP BY organization_id, COALESCE(payment_id, '00000000-0000-0000-0000-000000000000'),
                 COALESCE(refund_id, '00000000-0000-0000-0000-000000000000'), account, entry_type
        HAVING COUNT(*) > 1
    ) THEN
        RAISE WARNING 'payment_ledger holds duplicate postings: unique index uq_payment_ledger_posting not created (run the reconciliation report)';
    ELSE
        CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_ledger_posting ON payment_ledger (
            organization_id, COALESCE(payment_id, '00000000-0000-0000-0000-000000000000'),
            COALESCE(refund_id, '00000000-0000-0000-0000-000000000000'), account, entry_type
        ) WHERE status = 'posted';
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS uq_payment_ledger_posting;
ALTER TABLE payment_webhook_events DROP COLUMN IF EXISTS attempts;
ALTER TABLE payment_webhook_events DROP COLUMN IF EXISTS claimed_at;
