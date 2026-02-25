// Package manual implements the pg.PayoutGateway interface for manual payouts.
// No API calls are made. All operations succeed immediately with a "pending_manual" status.
// The admin is expected to process the payout externally.
package manual

import (
	"context"
	"encoding/json"
	"fmt"

	pg "github.com/KriaaCompany/pg-switcher-sdk"
)

// Adapter implements pg.PayoutGateway for manual payouts
type Adapter struct{}

// New creates a new manual PayoutGateway adapter
func New() *Adapter { return &Adapter{} }

// Name returns the gateway identifier
func (a *Adapter) Name() string { return "manual" }

// IsManual returns true — this adapter signals that admin handles payouts externally
func (a *Adapter) IsManual() bool { return true }

// CreateContact is a no-op for manual payouts
func (a *Adapter) CreateContact(_ context.Context, req pg.CreateContactRequest) (*pg.ContactResponse, error) {
	return &pg.ContactResponse{ContactID: "manual_" + req.ReferenceID}, nil
}

// UpdateContact is a no-op for manual payouts
func (a *Adapter) UpdateContact(_ context.Context, contactID string, _ pg.CreateContactRequest) (*pg.ContactResponse, error) {
	return &pg.ContactResponse{ContactID: contactID}, nil
}

// CreateFundAccount is a no-op for manual payouts
func (a *Adapter) CreateFundAccount(_ context.Context, req pg.CreateFundAccountRequest) (*pg.FundAccountResponse, error) {
	return &pg.FundAccountResponse{FundAccountID: "manual_" + req.ContactID}, nil
}

// InitiatePayout records a manual payout intent — no API call is made
func (a *Adapter) InitiatePayout(_ context.Context, req pg.InitiatePayoutRequest) (*pg.PayoutResponse, error) {
	return &pg.PayoutResponse{
		GatewayPayoutID: "manual_" + req.ReferenceID,
		Status:          "pending_manual",
	}, nil
}

// GetPayoutStatus returns pending_manual for all manual payout IDs
func (a *Adapter) GetPayoutStatus(_ context.Context, gatewayPayoutID string) (*pg.PayoutStatusResponse, error) {
	return &pg.PayoutStatusResponse{
		GatewayPayoutID: gatewayPayoutID,
		Status:          "pending_manual",
	}, nil
}

// VerifyWebhookSignature always returns false — manual payouts have no webhooks
func (a *Adapter) VerifyWebhookSignature(_ []byte, _ map[string]string) bool { return false }

// ParseWebhookEvent returns an error — manual payouts have no webhooks
func (a *Adapter) ParseWebhookEvent(payload []byte) (*pg.PayoutWebhookEvent, error) {
	var raw map[string]interface{}
	json.Unmarshal(payload, &raw)
	return nil, fmt.Errorf("manual: no webhooks for manual payout gateway")
}
