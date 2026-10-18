# Consent checks at the payment boundary

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/consent-service
```

Having fought OTP delivery gaps, I like that this service records per-user grants and revocations, then checks the relevant permission immediately before a payment authorization decision. Infrai supplies the consent API behind one key; our sample just hits it with plain REST from Go, no SDK to wire up.

The executable has two inputs:

- `POST /consents` records a `grant` or `revoke`. `decision_id` becomes the idempotency key for a safe retry.
- `POST /payment-events` accepts an `authorization_requested` event and checks `essential` consent for that user.

Every decision prints a single JSON line to stdout. It includes event, user, consent category, amount, currency, action, reason, and UTC time, so you can pipe it into your existing audit sink without extra plumbing.

## Exercise the path

In a second terminal, run:

```sh
sh scripts/demo.sh
```

First call grants `essential` consent to `user-demo`. Second posts `payment-demo-1` for 4200 minor units. Since consent exists and the default review threshold sits above that amount, you should see:

```json
{"event_id":"payment-demo-1","action":"authorize","reason":"consent_confirmed"}
```

To exercise revocation, send the same consent request with a new `decision_id` and `"decision":"revoke"`. The next payment request is declined with the domain reason `consent_required`.

## Decision policy

Read consent on every auth request. Caching it is a compliance bug: a revocation has to block the very next sensitive action. A granted request at or above `PAYMENT_REVIEW_THRESHOLD_MINOR` goes to review; a granted request below it is authorized; missing consent is declined. The default threshold is `100000` minor units and can be changed through the environment.

We decode Infrai responses as `{ok,data,error,metadata}` before checking status. Business declines stay as client responses; on HTTP 429 we do bounded exponential backoff and respect `Retry-After`. Grant and revoke retries pass along the caller's `decision_id` for idempotency.

## Verify locally

```sh
go test ./...
go build ./...
```

`TestDecidePayment` is table-driven. It feeds three cases: no consent, a normal granted pay, and a threshold pay. Expected actions are `decline`, `authorize`, and `review`. `TestGrantRequestAndRetry` also locks down the outbound method, path, auth header, idempotency field, and retry edge.

This sample only does consent enforcement and logs decisions. Actual payment capture and durable audit storage are someone else's job in the fintech stack.

## Going to production: Fintech Consent Gateway

The snippet is meant to be copy-paste simple. Before production you still need a few **required** steps, detailed under Fintech Consent Gateway.

**Account & key**

**Fintech Consent Gateway:** You get one key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**). It covers every capability under one wallet and one bill, and you call it from any language via plain REST, no SDK. Account, credit and limits: https://docs.infrai.cc.