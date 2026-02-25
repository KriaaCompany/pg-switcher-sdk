// Package razorpayx implements the pg.PayoutGateway interface using the RazorpayX API.
package razorpayx

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	rzp "github.com/razorpay/razorpay-go"
	rzpErrors "github.com/razorpay/razorpay-go/errors"

	pg "github.com/KriaaCompany/pg-switcher-sdk"
)

// Config holds RazorpayX payout credentials
type Config struct {
	KeyID         string
	KeySecret     string
	AccountNumber string
	WebhookSecret string
}

// Adapter wraps the RazorpayX API and implements pg.PayoutGateway
type Adapter struct {
	cfg    Config
	client *rzp.Client
}

// New creates a new RazorpayX PayoutGateway adapter
func New(cfg Config) *Adapter {
	return &Adapter{
		cfg:    cfg,
		client: rzp.NewClient(cfg.KeyID, cfg.KeySecret),
	}
}

// Name returns the gateway identifier
func (a *Adapter) Name() string { return "razorpayx" }

// IsManual returns false — RazorpayX uses the API
func (a *Adapter) IsManual() bool { return false }

// CreateContact creates a RazorpayX contact
func (a *Adapter) CreateContact(_ context.Context, req pg.CreateContactRequest) (*pg.ContactResponse, error) {
	body := map[string]interface{}{
		"name":         req.Name,
		"type":         "vendor",
		"reference_id": req.ReferenceID,
	}
	if req.Email != "" {
		body["email"] = req.Email
	}
	if req.Phone != "" {
		body["contact"] = req.Phone
	}

	result, err := a.client.Request.Post("/v1/contacts", body, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return nil, fmt.Errorf("razorpayx: create contact failed: %s", describeError(err))
	}

	id, ok := result["id"].(string)
	if !ok {
		return nil, fmt.Errorf("razorpayx: contact response missing id")
	}
	return &pg.ContactResponse{ContactID: id}, nil
}

// UpdateContact updates an existing RazorpayX contact
func (a *Adapter) UpdateContact(_ context.Context, contactID string, req pg.CreateContactRequest) (*pg.ContactResponse, error) {
	body := map[string]interface{}{
		"name": req.Name,
	}
	if req.Email != "" {
		body["email"] = req.Email
	}
	if req.Phone != "" {
		body["contact"] = req.Phone
	}

	result, err := a.client.Request.Patch(fmt.Sprintf("/v1/contacts/%s", contactID), body, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return nil, fmt.Errorf("razorpayx: update contact failed: %s", describeError(err))
	}

	id, ok := result["id"].(string)
	if !ok {
		id = contactID
	}
	return &pg.ContactResponse{ContactID: id}, nil
}

// CreateFundAccount creates a RazorpayX fund account (UPI or bank)
func (a *Adapter) CreateFundAccount(_ context.Context, req pg.CreateFundAccountRequest) (*pg.FundAccountResponse, error) {
	var body map[string]interface{}

	switch req.AccountType {
	case "vpa":
		body = map[string]interface{}{
			"contact_id":   req.ContactID,
			"account_type": "vpa",
			"vpa": map[string]interface{}{
				"address": req.VPA,
			},
		}
	case "bank_account":
		body = map[string]interface{}{
			"contact_id":   req.ContactID,
			"account_type": "bank_account",
			"bank_account": map[string]interface{}{
				"name":           req.AccountName,
				"account_number": req.AccountNumber,
				"ifsc":           req.IFSC,
			},
		}
	default:
		return nil, fmt.Errorf("razorpayx: unknown account type %q", req.AccountType)
	}

	result, err := a.client.FundAccount.Create(body, nil)
	if err != nil {
		return nil, fmt.Errorf("razorpayx: create fund account failed: %s", describeError(err))
	}

	id, ok := result["id"].(string)
	if !ok {
		return nil, fmt.Errorf("razorpayx: fund account response missing id")
	}
	return &pg.FundAccountResponse{FundAccountID: id}, nil
}

// InitiatePayout creates a RazorpayX payout
func (a *Adapter) InitiatePayout(_ context.Context, req pg.InitiatePayoutRequest) (*pg.PayoutResponse, error) {
	body := map[string]interface{}{
		"account_number":       a.cfg.AccountNumber,
		"fund_account_id":      req.FundAccountID,
		"amount":               req.Amount,
		"currency":             req.Currency,
		"mode":                 req.Mode,
		"purpose":              "payout",
		"queue_if_low_balance": true,
		"reference_id":         req.ReferenceID,
		"narration":            req.Narration,
	}

	extraHeaders := map[string]string{
		"Content-Type":         "application/json",
		"X-Payout-Idempotency": req.ReferenceID,
	}

	result, err := a.client.Request.Post("/v1/payouts", body, extraHeaders)
	if err != nil {
		return nil, fmt.Errorf("razorpayx: create payout failed: %s", describeError(err))
	}

	id, ok := result["id"].(string)
	if !ok {
		return nil, fmt.Errorf("razorpayx: payout response missing id")
	}
	status, _ := result["status"].(string)
	return &pg.PayoutResponse{GatewayPayoutID: id, Status: status}, nil
}

// GetPayoutStatus queries the status of a RazorpayX payout
func (a *Adapter) GetPayoutStatus(_ context.Context, gatewayPayoutID string) (*pg.PayoutStatusResponse, error) {
	result, err := a.client.Request.Get(fmt.Sprintf("/v1/payouts/%s", gatewayPayoutID), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("razorpayx: get payout status failed: %s", describeError(err))
	}
	status, _ := result["status"].(string)
	failureReason, _ := result["failure_reason"].(string)
	return &pg.PayoutStatusResponse{
		GatewayPayoutID: gatewayPayoutID,
		Status:          status,
		FailureReason:   failureReason,
	}, nil
}

// VerifyWebhookSignature verifies the X-Razorpayx-Signature header
func (a *Adapter) VerifyWebhookSignature(payload []byte, headers map[string]string) bool {
	sig := headers["x-razorpayx-signature"]
	if sig == "" {
		// Also try the payment webhook signature header (some setups share secrets)
		sig = headers["x-razorpay-signature"]
	}
	if sig == "" || a.cfg.WebhookSecret == "" {
		return false
	}
	h := hmac.New(sha256.New, []byte(a.cfg.WebhookSecret))
	h.Write(payload)
	expected := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}

// ParseWebhookEvent parses a RazorpayX webhook payload
func (a *Adapter) ParseWebhookEvent(payload []byte) (*pg.PayoutWebhookEvent, error) {
	var envelope struct {
		Event   string `json:"event"`
		Payload struct {
			Payout struct {
				Entity struct {
					ID            string `json:"id"`
					FailureReason string `json:"failure_reason"`
				} `json:"entity"`
			} `json:"payout"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("razorpayx: failed to parse webhook: %w", err)
	}

	evt := &pg.PayoutWebhookEvent{
		GatewayPayoutID: envelope.Payload.Payout.Entity.ID,
		FailureReason:   envelope.Payload.Payout.Entity.FailureReason,
	}

	switch {
	case envelope.Event == "payout.processed":
		evt.Type = pg.PayoutWebhookEventProcessed
	case strings.HasSuffix(envelope.Event, "payout.failed") || envelope.Event == "payout.rejected":
		evt.Type = pg.PayoutWebhookEventFailed
	case envelope.Event == "payout.reversed":
		evt.Type = pg.PayoutWebhookEventReversed
	default:
		evt.Type = pg.PayoutWebhookEventUnknown
	}

	return evt, nil
}

// describeError extracts a meaningful message from razorpay-go SDK errors
func describeError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if msg != "" {
		return msg
	}
	switch err.(type) {
	case *rzpErrors.BadRequestError:
		return "bad request (response body could not be parsed — check API credentials and payload)"
	case *rzpErrors.ServerError:
		return "server error from RazorpayX (response body could not be parsed)"
	case *rzpErrors.GatewayError:
		return "gateway error from RazorpayX (response body could not be parsed)"
	default:
		return "unknown error from RazorpayX (empty error message)"
	}
}
