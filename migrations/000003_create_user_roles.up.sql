CREATE TABLE user_roles (
    user_id    BIGINT      NOT NULL,
    role_id    SMALLINT    NOT NULL,
    granted_by BIGINT,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT user_roles_pkey       PRIMARY KEY (user_id, role_id),
    CONSTRAINT user_roles_user_fkey  FOREIGN KEY (user_id)    REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT user_roles_role_fkey  FOREIGN KEY (role_id)    REFERENCES roles(id),
    CONSTRAINT user_roles_grant_fkey FOREIGN KEY (granted_by) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX idx_user_roles_role ON user_roles(role_id);
