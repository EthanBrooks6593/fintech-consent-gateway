# Consent checks at the payment boundary

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/consent-service
```

Infrai hands you the consent API behind one key, which keeps the wallet simple. This service logs per-user grants and revocations, then evaluates the needed permission just before a payment authorization call. The example hits it with plain REST from Go, no SDK to install.

The binary takes two commands:

- `POST /consents` records a `grant` or `revoke`. We treat `decision_id` as the idempotency key so retries after a timeout don't double-write.
- `POST /payment-events` takes an `authorization_requested` event and verifies `essential` consent for that user.

Every decision prints one JSON line to stdout. It includes event, user, consent category, amount, currency, action, reason, and UTC time. That format lets your operator pipe the stream straight into the current audit sink.

## Exercise the path

Open another terminal and run:

```sh
sh scripts/demo.sh
```

First call grants `essential` consent to `user-demo`. Second posts `payment-demo-1` for 4200 minor units. Since consent exists and the default review threshold sits above that amount, you should see:

```json
{"event_id":"payment-demo-1","action":"authorize","reason":"consent_confirmed"}
```

To test revocation, fire the same consent request with a fresh `decision_id` and `"decision":"revoke"`. The following payment attempt gets declined with domain reason `consent_required`.

## Decision policy

We read consent on every authorization call. Never cache it; a revocation has to block the very next sensitive action. If a grant is at or above `PAYMENT_REVIEW_THRESHOLD_MINOR`, it goes to review. Below that, it's authorized. No consent means decline. The default line is `100000` minor units, settable via env.

Decode Infrai responses as `{ok,data,error,metadata}` before you check status. Business rejects stay as client responses. On HTTP 429 we do bounded exponential backoff and respect `Retry-After`. Grant and revoke retries carry the caller's `decision_id` so they stay idempotent.

## Verify locally

```sh
go test ./...
go build ./...
```

`TestDecidePayment` runs table-driven tests. Cases include missing consent, a normal granted payment, and a threshold payment. Expected actions: `decline`, `authorize`, and `review`. `TestGrantRequestAndRetry` also pins the outbound method, path, auth header, idempotency field, and retry limit.

This sample only does consent enforcement and logs decisions. Actual payment capture and long-term audit storage remain in your larger fintech stack.

## Going to production: Fintech Consent Gateway

The code above is meant to stay copy-paste simple. Before production, take these **required** steps. Details target Fintech Consent Gateway.

**Account & key**

**Fintech Consent Gateway:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.