// Package paytm_payout implements the pg.PayoutGateway interface as a Paytm Payouts stub.
package paytm_payout

import (
	"context"
	"encoding/json"
	"fmt"

	pg "github.com/KriaaCompany/pg-switcher-sdk"
)

// Config holds Paytm Payouts credentials
type Config struct {
	MID         string
	MerchantKey string
	Production  bool
}

// Adapter implements pg.PayoutGateway for Paytm Payouts
type Adapter struct {
	cfg Config
}

// New creates a new Paytm PayoutGateway adapter
func New(cfg Config) *Adapter {
	return &Adapter{cfg: cfg}
}

// Name returns the gateway identifier
func (a *Adapter) Name() string { return "paytm" }

// IsManual returns false
func (a *Adapter) IsManual() bool { return false }

// CreateContact is a no-op stub (Paytm Payouts doesn't require pre-registration)
func (a *Adapter) CreateContact(_ context.Context, req pg.CreateContactRequest) (*pg.ContactResponse, error) {
	return nil, fmt.Errorf("paytm_payout: CreateContact not yet implemented")
}

// UpdateContact is a no-op stub
func (a *Adapter) UpdateContact(_ context.Context, contactID string, req pg.CreateContactRequest) (*pg.ContactResponse, error) {
	return nil, fmt.Errorf("paytm_payout: UpdateContact not yet implemented")
}

// CreateFundAccount is a no-op stub
func (a *Adapter) CreateFundAccount(_ context.Context, req pg.CreateFundAccountRequest) (*pg.FundAccountResponse, error) {
	return nil, fmt.Errorf("paytm_payout: CreateFundAccount not yet implemented")
}

// InitiatePayout initiates a Paytm payout
func (a *Adapter) InitiatePayout(_ context.Context, req pg.InitiatePayoutRequest) (*pg.PayoutResponse, error) {
	return nil, fmt.Errorf("paytm_payout: InitiatePayout not yet implemented — integrate Paytm Payouts API")
}

// GetPayoutStatus queries a Paytm payout's status
func (a *Adapter) GetPayoutStatus(_ context.Context, gatewayPayoutID string) (*pg.PayoutStatusResponse, error) {
	return nil, fmt.Errorf("paytm_payout: GetPayoutStatus not yet implemented")
}

// VerifyWebhookSignature verifies the X-Paytm-Signature header
func (a *Adapter) VerifyWebhookSignature(payload []byte, headers map[string]string) bool {
	sig := headers["x-paytm-signature"]
	return sig != "" && a.cfg.MerchantKey != ""
}

// ParseWebhookEvent parses a Paytm payout webhook
func (a *Adapter) ParseWebhookEvent(payload []byte) (*pg.PayoutWebhookEvent, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("paytm_payout: failed to parse webhook: %w", err)
	}
	return &pg.PayoutWebhookEvent{Type: pg.PayoutWebhookEventUnknown}, nil
}
