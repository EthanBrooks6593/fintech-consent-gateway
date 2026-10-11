#!/bin/sh
set -eu

curl --fail-with-body -sS -X POST http://localhost:8080/consents \
  -H 'Content-Type: application/json' \
  -d '{"decision_id":"decision-demo-1","user_id":"user-demo","category":"essential","decision":"grant","source":"checkout"}'
printf '\n'
curl --fail-with-body -sS -X POST http://localhost:8080/payment-events \
  -H 'Content-Type: application/json' \
  -d '{"event_id":"payment-demo-1","user_id":"user-demo","kind":"authorization_requested","amount_minor":4200,"currency":"USD"}'
printf '\n'
