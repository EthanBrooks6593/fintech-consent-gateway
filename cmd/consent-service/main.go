package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"example.com/fintech-consent-gateway/consent"
)

type service struct {
	client          *consent.Client
	reviewThreshold int64
	logger          *log.Logger
}

type consentCommand struct {
	DecisionID string `json:"decision_id"`
	UserID     string `json:"user_id"`
	Category   string `json:"category"`
	Decision   string `json:"decision"`
	Source     string `json:"source"`
}

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	threshold := int64(100000)
	if raw := os.Getenv("PAYMENT_REVIEW_THRESHOLD_MINOR"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			log.Fatal("PAYMENT_REVIEW_THRESHOLD_MINOR must be a positive integer")
		}
		threshold = value
	}

	svc := &service{
		client:          &consent.Client{APIKey: apiKey, MaxRetries: 3},
		reviewThreshold: threshold,
		logger:          log.New(os.Stdout, "", 0),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /consents", svc.changeConsent)
	mux.HandleFunc("POST /payment-events", svc.handlePayment)

	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("consent service listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func (s *service) changeConsent(w http.ResponseWriter, r *http.Request) {
	var command consentCommand
	if err := decodeJSON(r, &command); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if command.DecisionID == "" || command.UserID == "" || command.Category == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "decision_id, user_id, and category are required"})
		return
	}

	var err error
	switch command.Decision {
	case "grant":
		err = s.client.Grant(r.Context(), command.UserID, command.Category, command.Source, command.DecisionID)
	case "revoke":
		err = s.client.Revoke(r.Context(), command.UserID, command.Category, command.DecisionID)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "decision must be grant or revoke"})
		return
	}
	if err != nil {
		s.writeClientError(w, err)
		return
	}

	s.notify(map[string]any{
		"at": time.Now().UTC(), "kind": "consent_changed", "decision_id": command.DecisionID,
		"user_id": command.UserID, "category": command.Category, "decision": command.Decision,
	})
	writeJSON(w, http.StatusOK, map[string]string{"decision_id": command.DecisionID, "status": "recorded"})
}

func (s *service) handlePayment(w http.ResponseWriter, r *http.Request) {
	var event consent.PaymentEvent
	if err := decodeJSON(r, &event); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if event.EventID == "" || event.UserID == "" || event.Kind != "authorization_requested" || event.AmountMinor <= 0 || len(event.Currency) != 3 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid event_id, user_id, kind, amount_minor, and currency are required"})
		return
	}

	// Read at authorization time so a revocation takes effect on the next payment.
	granted, err := s.client.Check(r.Context(), event.UserID, consent.PaymentCategory)
	if err != nil {
		s.writeClientError(w, err)
		return
	}
	decision := consent.DecidePayment(event, granted, s.reviewThreshold)
	s.notify(consent.NewAuditNotification(time.Now(), event, decision))
	writeJSON(w, http.StatusOK, decision)
}

func (s *service) notify(value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		s.logger.Printf(`{"kind":"audit_encoding_error"}`)
		return
	}
	s.logger.Print(string(raw))
}

func (s *service) writeClientError(w http.ResponseWriter, err error) {
	var apiErr *consent.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Code, "message": apiErr.Message})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "consent_service_error"})
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
