package consent

import "time"

const PaymentCategory = "essential"

type PaymentEvent struct {
	EventID     string `json:"event_id"`
	UserID      string `json:"user_id"`
	Kind        string `json:"kind"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}

type PaymentDecision struct {
	EventID string `json:"event_id"`
	Action  string `json:"action"`
	Reason  string `json:"reason"`
}

type AuditNotification struct {
	At          time.Time `json:"at"`
	EventID     string    `json:"event_id"`
	UserID      string    `json:"user_id"`
	Category    string    `json:"category"`
	EventKind   string    `json:"event_kind"`
	Decision    string    `json:"decision"`
	Reason      string    `json:"reason"`
	AmountMinor int64     `json:"amount_minor"`
	Currency    string    `json:"currency"`
}

func DecidePayment(event PaymentEvent, consentGranted bool, reviewThreshold int64) PaymentDecision {
	decision := PaymentDecision{EventID: event.EventID}
	switch {
	case !consentGranted:
		decision.Action = "decline"
		decision.Reason = "consent_required"
	case event.AmountMinor >= reviewThreshold:
		decision.Action = "review"
		decision.Reason = "amount_threshold"
	default:
		decision.Action = "authorize"
		decision.Reason = "consent_confirmed"
	}
	return decision
}

func NewAuditNotification(now time.Time, event PaymentEvent, decision PaymentDecision) AuditNotification {
	return AuditNotification{
		At: now.UTC(), EventID: event.EventID, UserID: event.UserID,
		Category: PaymentCategory, EventKind: event.Kind, Decision: decision.Action,
		Reason: decision.Reason, AmountMinor: event.AmountMinor, Currency: event.Currency,
	}
}
