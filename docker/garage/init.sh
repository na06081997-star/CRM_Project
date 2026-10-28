#!/bin/sh
# Idempotent bootstrap of a single-node Garage through its admin API (v1).
set -eu

apk add --no-cache -q curl jq >/dev/null

API="http://garage:3903/v1"
AUTH="Authorization: Bearer ${GARAGE_ADMIN_TOKEN}"

api() { # method path [json-body]
  if [ $# -ge 3 ]; then
    curl -fsS -X "$1" -H "$AUTH" -H 'Content-Type: application/json' -d "$3" "$API$2"
  else
    curl -fsS -X "$1" -H "$AUTH" "$API$2"
  fi
}

echo "[garage-init] waiting for admin API"
i=0
until api GET /status >/dev/null 2>&1; do
  i=$((i + 1)); [ "$i" -gt 30 ] && { echo "admin API unreachable"; exit 1; }
  sleep 2
done

NODE_ID=$(api GET /status | jq -r '.node')
ROLE=$(api GET /layout | jq -r --arg n "$NODE_ID" '.roles[] | select(.id == $n) | .id' || true)

if [ -z "$ROLE" ]; then
  echo "[garage-init] assigning layout to node $NODE_ID"
  api POST /layout "[{\"id\":\"$NODE_ID\",\"zone\":\"dc1\",\"capacity\":10000000000,\"tags\":[\"crm\"]}]" >/dev/null
  VERSION=$(api GET /layout | jq -r '.version + 1')
  api POST /layout/apply "{\"version\":$VERSION}" >/dev/null
fi

if ! api GET "/key?id=${S3_ACCESS_KEY}" >/dev/null 2>&1; then
  echo "[garage-init] importing S3 key"
  api POST /key/import "{\"accessKeyId\":\"$S3_ACCESS_KEY\",\"secretAccessKey\":\"$S3_SECRET_KEY\",\"name\":\"crm-backend\"}" >/dev/null
fi

BUCKET_ID=$(api GET "/bucket?globalAlias=${S3_BUCKET}" 2>/dev/null | jq -r '.id' || true)
if [ -z "$BUCKET_ID" ] || [ "$BUCKET_ID" = "null" ]; then
  echo "[garage-init] creating bucket $S3_BUCKET"
  BUCKET_ID=$(api POST /bucket "{\"globalAlias\":\"$S3_BUCKET\"}" | jq -r '.id')
fi

api POST /bucket/allow "{\"bucketId\":\"$BUCKET_ID\",\"accessKeyId\":\"$S3_ACCESS_KEY\",\"permissions\":{\"read\":true,\"write\":true,\"owner\":true}}" >/dev/null
echo "[garage-init] ready (bucket=$S3_BUCKET)"
