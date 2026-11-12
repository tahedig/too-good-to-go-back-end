DROP TABLE   IF EXISTS refunds;
DROP TRIGGER IF EXISTS trg_payments_updated ON payments;
DROP TABLE   IF EXISTS payments;
DROP TYPE    IF EXISTS refund_status;
DROP TYPE    IF EXISTS payment_status;
