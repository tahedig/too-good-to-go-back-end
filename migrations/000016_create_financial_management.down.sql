DROP TRIGGER IF EXISTS trg_payout_requests_updated      ON payout_requests;
DROP TABLE   IF EXISTS payout_requests;
DROP TRIGGER IF EXISTS trg_vendor_subscriptions_updated ON vendor_subscriptions;
DROP TABLE   IF EXISTS vendor_subscriptions;
DROP TRIGGER IF EXISTS trg_subscription_plans_updated   ON subscription_plans;
DROP TABLE   IF EXISTS subscription_plans;
DROP TYPE    IF EXISTS payout_status;
DROP TYPE    IF EXISTS subscription_status;
