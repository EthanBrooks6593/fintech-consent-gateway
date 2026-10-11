package consent

import "testing"

func TestDecidePayment(t *testing.T) {
	tests := []struct {
		name       string
		granted    bool
		amount     int64
		wantAction string
		wantReason string
	}{
		{name: "missing consent declines", granted: false, amount: 1200, wantAction: "decline", wantReason: "consent_required"},
		{name: "ordinary payment authorizes", granted: true, amount: 1200, wantAction: "authorize", wantReason: "consent_confirmed"},
		{name: "threshold payment needs review", granted: true, amount: 100000, wantAction: "review", wantReason: "amount_threshold"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecidePayment(PaymentEvent{EventID: "evt-42", AmountMinor: tt.amount}, tt.granted, 100000)
			if got.Action != tt.wantAction || got.Reason != tt.wantReason {
				t.Fatalf("DecidePayment() = %s/%s, want %s/%s", got.Action, got.Reason, tt.wantAction, tt.wantReason)
			}
		})
	}
}
