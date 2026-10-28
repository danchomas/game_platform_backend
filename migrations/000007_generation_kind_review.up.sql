-- Фоновая проверка логики готовой игры ревьюером — отдельный вид генерации.
ALTER TABLE generations DROP CONSTRAINT IF EXISTS generations_kind_check;
ALTER TABLE generations ADD CONSTRAINT generations_kind_check
    CHECK (kind IN ('plan','revise','build','edit','review'));
