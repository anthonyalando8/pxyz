\c pxyz_admin;

BEGIN;

-- rbac modules (hierarchical, parent-child)
CREATE TABLE rbac_modules (
  id BIGSERIAL PRIMARY KEY,
  parent_id BIGINT REFERENCES rbac_modules(id) ON DELETE CASCADE,  -- parent module
  code TEXT NOT NULL UNIQUE,
  name TEXT,
  meta JSONB,
  is_active BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMPTZ DEFAULT now(),
  created_by BIGINT,
  updated_at TIMESTAMPTZ,
  updated_by BIGINT
);

-- rbac submodules (tied to modules, not hierarchical for now)
CREATE TABLE rbac_submodules (
  id BIGSERIAL PRIMARY KEY,
  module_id BIGINT NOT NULL REFERENCES rbac_modules(id) ON DELETE CASCADE,
  code TEXT NOT NULL,
  name TEXT,
  UNIQUE(module_id, code),
  meta JSONB,
  is_active BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMPTZ DEFAULT now(),
  created_by BIGINT,
  updated_at TIMESTAMPTZ,
  updated_by BIGINT
);

-- rbac permission types
CREATE TABLE rbac_permission_types (
  id BIGSERIAL PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  description TEXT,
  is_active BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMPTZ DEFAULT now(),
  created_by BIGINT,
  updated_at TIMESTAMPTZ,
  updated_by BIGINT
);

-- rbac roles
CREATE TABLE rbac_roles (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  description TEXT,
  is_active BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMPTZ DEFAULT now(),
  created_by BIGINT,
  updated_at TIMESTAMPTZ,
  updated_by BIGINT
);

-- rbac role -> permissions
CREATE TABLE rbac_role_permissions (
  id BIGSERIAL PRIMARY KEY,
  role_id BIGINT NOT NULL REFERENCES rbac_roles(id) ON DELETE CASCADE,
  module_id BIGINT NOT NULL REFERENCES rbac_modules(id) ON DELETE CASCADE,
  submodule_id BIGINT REFERENCES rbac_submodules(id) ON DELETE CASCADE,
  permission_type_id BIGINT NOT NULL REFERENCES rbac_permission_types(id),
  allow BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ DEFAULT now(),
  created_by BIGINT,
  updated_at TIMESTAMPTZ,
  updated_by BIGINT,

  -- enforce no exact duplicates at full granularity
  UNIQUE(role_id, module_id, submodule_id, permission_type_id)
);

-- Prevent duplicate module-level permissions (when submodule_id IS NULL)
CREATE UNIQUE INDEX uniq_role_module_permission
ON rbac_role_permissions(role_id, module_id, permission_type_id)
WHERE submodule_id IS NULL;

-- Prevent duplicate submodule-level permissions (when submodule_id IS NOT NULL)
CREATE UNIQUE INDEX uniq_role_module_submodule_permission
ON rbac_role_permissions(role_id, module_id, submodule_id, permission_type_id)
WHERE submodule_id IS NOT NULL;

-- rbac user roles (assigning roles to users)
CREATE TABLE rbac_user_roles (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id BIGINT NOT NULL REFERENCES rbac_roles(id) ON DELETE CASCADE,
  assigned_by BIGINT,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ,
  updated_by BIGINT,
  CONSTRAINT uq_user_role UNIQUE (user_id, role_id)
);


-- rbac user-specific overrides
CREATE TABLE rbac_user_permissions_override (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  module_id BIGINT NOT NULL REFERENCES rbac_modules(id),
  submodule_id BIGINT REFERENCES rbac_submodules(id),
  permission_type_id BIGINT NOT NULL REFERENCES rbac_permission_types(id),
  allow BOOLEAN NOT NULL,
  UNIQUE(user_id, module_id, submodule_id, permission_type_id),
  created_at TIMESTAMPTZ DEFAULT now(),
  created_by BIGINT,
  updated_at TIMESTAMPTZ,
  updated_by BIGINT
);

-- rbac audit log (records who did what and when)
CREATE TABLE rbac_permissions_audit (
  id BIGSERIAL PRIMARY KEY,
  actor_id BIGINT,         -- can be user/system
  object_type TEXT,
  object_id BIGINT,
  action TEXT,
  payload JSONB,
  created_at TIMESTAMPTZ DEFAULT now()
);


COMMIT;