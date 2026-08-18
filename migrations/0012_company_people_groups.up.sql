-- Establish explicit company ownership and Alzette-local access groups without
-- treating legacy portal roles or identity-provider claims as authority.

CREATE TABLE organisation_people (
    id                       TEXT PRIMARY KEY,
    organisation_id          TEXT NOT NULL REFERENCES organisations(id) ON DELETE RESTRICT,
    user_id                  TEXT NOT NULL REFERENCES human_users(id) ON DELETE RESTRICT,
    enabled                  BOOLEAN NOT NULL DEFAULT true,
    authorisation_generation BIGINT NOT NULL DEFAULT 1,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organisation_id, user_id),
    UNIQUE (organisation_id, id),
    CHECK (authorisation_generation > 0)
);

CREATE TABLE organisation_ownerships (
    id                TEXT PRIMARY KEY,
    organisation_id   TEXT NOT NULL,
    person_id         TEXT NOT NULL,
    started_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at          TIMESTAMPTZ,
    change_kind       TEXT NOT NULL,
    actor_type        TEXT NOT NULL,
    actor_id          TEXT NOT NULL,
    evidence_ref      TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organisation_id, person_id)
        REFERENCES organisation_people(organisation_id, id) ON DELETE RESTRICT,
    CHECK (ended_at IS NULL OR ended_at >= started_at),
    CHECK (change_kind IN ('initial', 'transfer', 'recovery')),
    CHECK (actor_type IN ('human_user', 'operator')),
    CHECK (length(actor_id) BETWEEN 1 AND 255),
    CHECK (evidence_ref ~ '^[A-Za-z0-9][A-Za-z0-9 ._:/-]{0,254}$')
);

CREATE UNIQUE INDEX organisation_ownerships_one_current_0012_idx
    ON organisation_ownerships(organisation_id)
    WHERE ended_at IS NULL;

CREATE TABLE access_groups (
    id               TEXT PRIMARY KEY,
    organisation_id  TEXT NOT NULL,
    project_id       TEXT NOT NULL,
    environment_id   TEXT NOT NULL,
    name             TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    enabled          BOOLEAN NOT NULL DEFAULT true,
    created_by       TEXT NOT NULL REFERENCES human_users(id) ON DELETE RESTRICT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organisation_id, project_id, environment_id)
        REFERENCES environments(organisation_id, project_id, id) ON DELETE RESTRICT,
    UNIQUE (organisation_id, id),
    UNIQUE (organisation_id, project_id, environment_id, id),
    CHECK (name ~ '^[A-Za-z0-9][A-Za-z0-9 ._:-]{0,126}[A-Za-z0-9]$' OR name ~ '^[A-Za-z0-9]$'),
    CHECK (length(description) <= 1000)
);

CREATE UNIQUE INDEX access_groups_name_0012_idx
    ON access_groups(organisation_id, lower(name));

CREATE TABLE access_group_people (
    organisation_id  TEXT NOT NULL,
    project_id       TEXT NOT NULL,
    environment_id   TEXT NOT NULL,
    group_id         TEXT NOT NULL,
    person_id        TEXT NOT NULL,
    created_by       TEXT NOT NULL REFERENCES human_users(id) ON DELETE RESTRICT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, person_id),
    FOREIGN KEY (organisation_id, project_id, environment_id, group_id)
        REFERENCES access_groups(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, person_id)
        REFERENCES organisation_people(organisation_id, id) ON DELETE RESTRICT
);

CREATE TABLE access_group_models (
    organisation_id  TEXT NOT NULL,
    project_id       TEXT NOT NULL,
    environment_id   TEXT NOT NULL,
    group_id         TEXT NOT NULL,
    route_id         TEXT NOT NULL,
    created_by       TEXT NOT NULL REFERENCES human_users(id) ON DELETE RESTRICT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, route_id),
    FOREIGN KEY (organisation_id, project_id, environment_id, group_id)
        REFERENCES access_groups(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (organisation_id, project_id, environment_id, route_id)
        REFERENCES tenant_routes(organisation_id, project_id, environment_id, id) ON DELETE RESTRICT
);

CREATE INDEX access_group_people_person_0012_idx
    ON access_group_people(organisation_id, person_id, group_id);
CREATE INDEX access_group_models_route_0012_idx
    ON access_group_models(organisation_id, route_id, group_id);

CREATE OR REPLACE FUNCTION protect_current_owner_person_0012() RETURNS trigger AS $$
BEGIN
    IF OLD.enabled AND NOT NEW.enabled AND EXISTS (
        SELECT 1
          FROM organisation_ownerships ownership
         WHERE ownership.organisation_id = OLD.organisation_id
           AND ownership.person_id = OLD.id
           AND ownership.ended_at IS NULL
    ) THEN
        RAISE EXCEPTION 'current company owner cannot be disabled'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER organisation_people_protect_current_owner_0012
BEFORE UPDATE OF enabled ON organisation_people
FOR EACH ROW EXECUTE FUNCTION protect_current_owner_person_0012();

CREATE OR REPLACE FUNCTION protect_current_owner_user_0012() RETURNS trigger AS $$
BEGIN
    IF OLD.enabled AND NOT NEW.enabled AND EXISTS (
        SELECT 1
          FROM organisation_people person
          JOIN organisation_ownerships ownership
            ON ownership.organisation_id = person.organisation_id
           AND ownership.person_id = person.id
           AND ownership.ended_at IS NULL
         WHERE person.user_id = OLD.id
    ) THEN
        RAISE EXCEPTION 'current company owner account cannot be disabled'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER human_users_protect_current_owner_0012
BEFORE UPDATE OF enabled ON human_users
FOR EACH ROW EXECUTE FUNCTION protect_current_owner_user_0012();
