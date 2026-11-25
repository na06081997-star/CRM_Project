#!/bin/sh
# One-shot: creates OpenSign's secrets on the first start of an instance, in
# the opensign-secrets volume. Nobody types or copies them:
#   master_key        Parse master key, read by OpenSign and by the CRM backend
#   signing.pfx.b64   self-signed certificate that seals the signed PDFs
#   signing.pass      its passphrase
# Idempotent: existing files are kept (rotating the master key would cut the
# CRM off its OpenSign account and invalidate file links).
set -eu
cd /secrets

rand() { tr -dc 'A-Za-z0-9' </dev/urandom | head -c "$1"; }

if [ ! -s master_key ]; then
  rand 40 > master_key
  echo "[opensign-init] master key created"
fi
# The backend runs as distroless "nonroot" (65532).
chown 65532:65532 master_key
chmod 0400 master_key

if [ ! -s signing.pfx.b64 ] || [ ! -s signing.pass ]; then
  apk add --no-cache -q openssl >/dev/null
  rand 32 > signing.pass
  tmp=$(mktemp -d)
  openssl req -x509 -newkey rsa:2048 -sha256 -days 3650 -nodes \
    -keyout "$tmp/key.pem" -out "$tmp/cert.pem" \
    -subj "/CN=${OPENSIGN_CERT_NAME:-CRM document signing}/O=${OPENSIGN_CERT_ORG:-CRM}" 2>/dev/null
  # 3DES/SHA1 PKCS#12: the format node-forge (OpenSign's PDF signer) reads.
  openssl pkcs12 -export -inkey "$tmp/key.pem" -in "$tmp/cert.pem" \
    -certpbe PBE-SHA1-3DES -keypbe PBE-SHA1-3DES -macalg sha1 \
    -passout "file:signing.pass" -out "$tmp/signing.pfx"
  base64 "$tmp/signing.pfx" | tr -d '\n' > signing.pfx.b64
  rm -rf "$tmp"
  chmod 0400 signing.pass signing.pfx.b64
  echo "[opensign-init] signing certificate created"
fi
echo "[opensign-init] ready"
