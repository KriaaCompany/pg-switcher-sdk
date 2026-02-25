package pg

import (
	"context"
	"fmt"
)

// GatewayResolver is called on every operation to determine which gateway to use.
// This allows admin config changes to take effect immediately without restart.
type GatewayResolver func(ctx context.Context) (string, error)

// --- DynamicPaymentSwitcher ---

// DynamicPaymentSwitcher resolves the active PaymentGateway at request time.
// It implements PaymentGateway and delegates all calls to the resolved adapter.
type DynamicPaymentSwitcher struct {
	gateways map[string]PaymentGateway
	resolver GatewayResolver
}

// NewDynamicPaymentSwitcher creates a DynamicPaymentSwitcher.
func NewDynamicPaymentSwitcher(gateways map[string]PaymentGateway, resolver GatewayResolver) *DynamicPaymentSwitcher {
	return &DynamicPaymentSwitcher{gateways: gateways, resolver: resolver}
}

func (s *DynamicPaymentSwitcher) resolve(ctx context.Context) (PaymentGateway, error) {
	name, err := s.resolver(ctx)
	if err != nil {
		return nil, fmt.Errorf("pg-switcher: resolver error: %w", err)
	}
	gw, ok := s.gateways[name]
	if !ok {
		return nil, fmt.Errorf("pg-switcher: payment gateway %q not registered", name)
	}
	return gw, nil
}

func (s *DynamicPaymentSwitcher) Name() string { return "dynamic" }

func (s *DynamicPaymentSwitcher) CreateOrder(ctx context.Context, req CreateOrderRequest) (*CreateOrderResponse, error) {
	gw, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return gw.CreateOrder(ctx, req)
}

func (s *DynamicPaymentSwitcher) VerifyPayment(ctx context.Context, req VerifyPaymentRequest) (bool, error) {
	gw, err := s.resolve(ctx)
	if err != nil {
		return false, err
	}
	return gw.VerifyPayment(ctx, req)
}

func (s *DynamicPaymentSwitcher) GetPaymentStatus(ctx context.Context, gatewayOrderID string) (*PaymentStatus, error) {
	gw, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return gw.GetPaymentStatus(ctx, gatewayOrderID)
}

func (s *DynamicPaymentSwitcher) InitiateRefund(ctx context.Context, req RefundRequest) (*RefundResponse, error) {
	gw, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return gw.InitiateRefund(ctx, req)
}

func (s *DynamicPaymentSwitcher) VerifyWebhookSignature(payload []byte, headers map[string]string) bool {
	// For webhook verification we try all registered gateways — the request context
	// is not available in webhook handlers that don't know the gateway yet.
	// The first gateway whose signature verification passes wins.
	for _, gw := range s.gateways {
		if gw.VerifyWebhookSignature(payload, headers) {
			return true
		}
	}
	return false
}

func (s *DynamicPaymentSwitcher) ParseWebhookEvent(payload []byte) (*WebhookEvent, error) {
	// Use background context since this is called from a webhook handler
	ctx := context.Background()
	gw, err := s.resolve(ctx)
	if err != nil {
		// Fallback: try each gateway
		for _, gw := range s.gateways {
			evt, err := gw.ParseWebhookEvent(payload)
			if err == nil {
				return evt, nil
			}
		}
		return nil, fmt.Errorf("pg-switcher: unable to parse webhook event: %w", err)
	}
	return gw.ParseWebhookEvent(payload)
}

func (s *DynamicPaymentSwitcher) ClientCredentials() map[string]interface{} {
	ctx := context.Background()
	gw, err := s.resolve(ctx)
	if err != nil {
		return map[string]interface{}{}
	}
	return gw.ClientCredentials()
}

// ActiveGatewayName resolves and returns the name of the currently active payment gateway.
func (s *DynamicPaymentSwitcher) ActiveGatewayName(ctx context.Context) (string, error) {
	gw, err := s.resolve(ctx)
	if err != nil {
		return "", err
	}
	return gw.Name(), nil
}

// --- DynamicPayoutSwitcher ---

// DynamicPayoutSwitcher resolves the active PayoutGateway at request time.
type DynamicPayoutSwitcher struct {
	gateways map[string]PayoutGateway
	resolver GatewayResolver
}

// NewDynamicPayoutSwitcher creates a DynamicPayoutSwitcher.
func NewDynamicPayoutSwitcher(gateways map[string]PayoutGateway, resolver GatewayResolver) *DynamicPayoutSwitcher {
	return &DynamicPayoutSwitcher{gateways: gateways, resolver: resolver}
}

func (s *DynamicPayoutSwitcher) resolve(ctx context.Context) (PayoutGateway, error) {
	name, err := s.resolver(ctx)
	if err != nil {
		return nil, fmt.Errorf("pg-switcher: resolver error: %w", err)
	}
	gw, ok := s.gateways[name]
	if !ok {
		return nil, fmt.Errorf("pg-switcher: payout gateway %q not registered", name)
	}
	return gw, nil
}

func (s *DynamicPayoutSwitcher) Name() string { return "dynamic" }

func (s *DynamicPayoutSwitcher) CreateContact(ctx context.Context, req CreateContactRequest) (*ContactResponse, error) {
	gw, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return gw.CreateContact(ctx, req)
}

func (s *DynamicPayoutSwitcher) UpdateContact(ctx context.Context, contactID string, req CreateContactRequest) (*ContactResponse, error) {
	gw, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return gw.UpdateContact(ctx, contactID, req)
}

func (s *DynamicPayoutSwitcher) CreateFundAccount(ctx context.Context, req CreateFundAccountRequest) (*FundAccountResponse, error) {
	gw, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return gw.CreateFundAccount(ctx, req)
}

func (s *DynamicPayoutSwitcher) InitiatePayout(ctx context.Context, req InitiatePayoutRequest) (*PayoutResponse, error) {
	gw, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return gw.InitiatePayout(ctx, req)
}

func (s *DynamicPayoutSwitcher) GetPayoutStatus(ctx context.Context, gatewayPayoutID string) (*PayoutStatusResponse, error) {
	gw, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return gw.GetPayoutStatus(ctx, gatewayPayoutID)
}

func (s *DynamicPayoutSwitcher) VerifyWebhookSignature(payload []byte, headers map[string]string) bool {
	for _, gw := range s.gateways {
		if gw.VerifyWebhookSignature(payload, headers) {
			return true
		}
	}
	return false
}

func (s *DynamicPayoutSwitcher) ParseWebhookEvent(payload []byte) (*PayoutWebhookEvent, error) {
	ctx := context.Background()
	gw, err := s.resolve(ctx)
	if err != nil {
		for _, gw := range s.gateways {
			evt, err := gw.ParseWebhookEvent(payload)
			if err == nil {
				return evt, nil
			}
		}
		return nil, fmt.Errorf("pg-switcher: unable to parse payout webhook event: %w", err)
	}
	return gw.ParseWebhookEvent(payload)
}

func (s *DynamicPayoutSwitcher) IsManual() bool {
	ctx := context.Background()
	gw, err := s.resolve(ctx)
	if err != nil {
		return false
	}
	return gw.IsManual()
}

// ActiveGatewayName resolves and returns the name of the currently active payout gateway.
func (s *DynamicPayoutSwitcher) ActiveGatewayName(ctx context.Context) (string, error) {
	gw, err := s.resolve(ctx)
	if err != nil {
		return "", err
	}
	return gw.Name(), nil
}
