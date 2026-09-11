-- Allow one device row per (installation_id, provider) so iOS can
-- register both apns and apns_voip under the same installation_id.
ALTER TABLE devices DROP CONSTRAINT IF EXISTS devices_installation_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_installation_provider
  ON devices (installation_id, provider);
