-- 0005_document_types.sql
-- Typed document uploads (Aadhaar offline-KYC, PAN, bank statement, ...) plus a
-- masked-metadata column. masked_meta only ever holds non-sensitive derived
-- values (e.g. Aadhaar last-4) — never a full Aadhaar number. Idempotent.

ALTER TABLE documents ADD COLUMN IF NOT EXISTS doc_type    TEXT  NOT NULL DEFAULT 'other';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS masked_meta JSONB NOT NULL DEFAULT '{}'::jsonb;
