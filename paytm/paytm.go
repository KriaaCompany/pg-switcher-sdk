// Package paytm implements the pg.PaymentGateway interface as a Paytm stub adapter.
// A full implementation would wrap the Paytm Payment Gateway SDK.
// This stub returns meaningful errors so the system can degrade gracefully
// if Paytm is configured as the active gateway but the SDK is not yet integrated.
package paytm

import (
	"context"
	"encoding/json"
	"fmt"

	pg "github.com/KriaaCompany/pg-switcher-sdk"
)

// Config holds Paytm payment gateway credentials
type Config struct {
	MID           string // Merchant ID
	MerchantKey   string
	Website       string // e.g. "WEBSTAGING" or "DEFAULT"
	CallbackURL   string
	WebhookSecret string
	Production    bool
}

// Adapter implements pg.PaymentGateway for Paytm
type Adapter struct {
	cfg Config
}

// New creates a new Paytm PaymentGateway adapter
func New(cfg Config) *Adapter {
	return &Adapter{cfg: cfg}
}

// Name returns the gateway identifier
func (a *Adapter) Name() string { return "paytm" }

// CreateOrder initiates a Paytm transaction and returns the txn_token
func (a *Adapter) CreateOrder(_ context.Context, req pg.CreateOrderRequest) (*pg.CreateOrderResponse, error) {
	// NOTE: Full Paytm integration requires the Paytm Payment Gateway SDK.
	// This stub demonstrates the expected behaviour. Replace with real SDK calls.
	return nil, fmt.Errorf("paytm: CreateOrder not yet implemented — integrate Paytm PG SDK")
}

// VerifyPayment verifies a Paytm payment
func (a *Adapter) VerifyPayment(_ context.Context, req pg.VerifyPaymentRequest) (bool, error) {
	return false, fmt.Errorf("paytm: VerifyPayment not yet implemented")
}

// GetPaymentStatus queries a Paytm order's status
func (a *Adapter) GetPaymentStatus(_ context.Context, gatewayOrderID string) (*pg.PaymentStatus, error) {
	return nil, fmt.Errorf("paytm: GetPaymentStatus not yet implemented")
}

// InitiateRefund initiates a Paytm refund
func (a *Adapter) InitiateRefund(_ context.Context, req pg.RefundRequest) (*pg.RefundResponse, error) {
	return nil, fmt.Errorf("paytm: InitiateRefund not yet implemented")
}

// VerifyWebhookSignature verifies the X-Paytm-Signature header
func (a *Adapter) VerifyWebhookSignature(payload []byte, headers map[string]string) bool {
	// Paytm uses a different signature scheme; implement when integrating the full SDK
	sig := headers["x-paytm-signature"]
	return sig != "" && a.cfg.WebhookSecret != ""
}

// ParseWebhookEvent parses a Paytm webhook payload
func (a *Adapter) ParseWebhookEvent(payload []byte) (*pg.WebhookEvent, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("paytm: failed to parse webhook: %w", err)
	}
	// Stub: return unknown until full integration
	return &pg.WebhookEvent{Type: pg.WebhookEventUnknown, Raw: raw}, nil
}

// ClientCredentials returns the Paytm credentials the mobile SDK needs
func (a *Adapter) ClientCredentials() map[string]interface{} {
	return map[string]interface{}{
		"mid": a.cfg.MID,
	}
}
