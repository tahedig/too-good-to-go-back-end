DROP TRIGGER  IF EXISTS trg_users_updated ON users;
DROP TABLE    IF EXISTS users;
DROP FUNCTION IF EXISTS fn_set_updated_at();
DROP TYPE     IF EXISTS user_status;
