-- Contracts signed through the self-hosted OpenSign (decision of 25/11/2026).
-- A digital contract keeps its CRM link (/sign/<token>); when OpenSign is
-- configured the page sends the signer to OpenSign's document. Without it,
-- or if OpenSign was unreachable when the contract was sent, the built-in
-- signature is used as before.
ALTER TABLE contracts
    ADD COLUMN opensign_doc_id text,
    ADD COLUMN opensign_url    text NOT NULL DEFAULT '';

CREATE INDEX contracts_opensign_pending_idx ON contracts (created_at)
    WHERE status = 'pending' AND opensign_doc_id IS NOT NULL;
