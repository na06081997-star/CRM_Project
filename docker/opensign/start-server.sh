#!/bin/sh
# Starts OpenSign's server with the secrets created by opensign-init.
set -eu
MASTER_KEY="$(cat /secrets/master_key)"
PFX_BASE64="$(cat /secrets/signing.pfx.b64)"
PASS_PHRASE="$(cat /secrets/signing.pass)"
export MASTER_KEY PFX_BASE64 PASS_PHRASE
cd /usr/src/app
exec npm start
