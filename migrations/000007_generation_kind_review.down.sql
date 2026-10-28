DELETE FROM generations WHERE kind = 'review';
ALTER TABLE generations DROP CONSTRAINT IF EXISTS generations_kind_check;
ALTER TABLE generations ADD CONSTRAINT generations_kind_check
    CHECK (kind IN ('plan','revise','build','edit'));
