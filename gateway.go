package pg

import "context"

// WebhookEventType represents a payment webhook event type
type WebhookEventType string

const (
	WebhookEventPaymentSuccess  WebhookEventType = "payment.success"
	WebhookEventPaymentFailed   WebhookEventType = "payment.failed"
	WebhookEventOrderPaid       WebhookEventType = "order.paid"
	WebhookEventRefundSuccess   WebhookEventType = "refund.success"
	WebhookEventRefundFailed    WebhookEventType = "refund.failed"
	WebhookEventDisputeCreated  WebhookEventType = "dispute.created"
	WebhookEventDisputeWon      WebhookEventType = "dispute.won"
	WebhookEventDisputeLost     WebhookEventType = "dispute.lost"
	WebhookEventDisputeClosed   WebhookEventType = "dispute.closed"
	WebhookEventUnknown         WebhookEventType = "unknown"
)

// CreateOrderRequest contains fields for creating a payment order
type CreateOrderRequest struct {
	Amount      int64             // in smallest currency unit (paise)
	Currency    string            // e.g. "INR"
	Receipt     string            // booking ref or similar
	Notes       map[string]string // arbitrary key-value notes
}

// CreateOrderResponse is returned after successfully creating an order
type CreateOrderResponse struct {
	GatewayOrderID string            // gateway-specific order/transaction ID
	Amount         int64
	Currency       string
	Notes          map[string]string
	// Extra contains gateway-specific fields (e.g. txn_token for Paytm)
	Extra map[string]interface{}
}

// VerifyPaymentRequest contains fields for verifying a payment
type VerifyPaymentRequest struct {
	GatewayOrderID   string
	GatewayPaymentID string
	Signature        string
}

// PaymentStatus represents the current status of a payment from the gateway
type PaymentStatus struct {
	GatewayOrderID   string
	GatewayPaymentID string
	Status           string // gateway-specific status string
	Paid             bool
}

// RefundRequest contains fields for initiating a refund
type RefundRequest struct {
	GatewayPaymentID string
	Amount           int64
	Notes            map[string]string
}

// RefundResponse is returned after initiating a refund
type RefundResponse struct {
	RefundID string
	Amount   int64
	Status   string
}

// WebhookEvent is a normalised representation of a payment gateway webhook event
type WebhookEvent struct {
	Type             WebhookEventType
	GatewayOrderID   string
	GatewayPaymentID string
	RefundID         string
	DisputeID        string
	Amount           int64
	Currency         string
	FailureReason    string
	// Raw contains the original parsed payload for gateway-specific handling
	Raw map[string]interface{}
}

// PaymentGateway is the common interface that all payment gateway adapters implement
type PaymentGateway interface {
	// Name returns the unique gateway identifier (e.g. "razorpay", "paytm")
	Name() string

	// CreateOrder creates a new payment order/transaction on the gateway
	CreateOrder(ctx context.Context, req CreateOrderRequest) (*CreateOrderResponse, error)

	// VerifyPayment verifies that a completed payment is authentic
	VerifyPayment(ctx context.Context, req VerifyPaymentRequest) (bool, error)

	// GetPaymentStatus queries the current status of an order from the gateway
	GetPaymentStatus(ctx context.Context, gatewayOrderID string) (*PaymentStatus, error)

	// InitiateRefund starts a refund for a completed payment
	InitiateRefund(ctx context.Context, req RefundRequest) (*RefundResponse, error)

	// VerifyWebhookSignature returns true if the webhook payload is authentic
	VerifyWebhookSignature(payload []byte, headers map[string]string) bool

	// ParseWebhookEvent parses a raw webhook payload into a normalised WebhookEvent
	ParseWebhookEvent(payload []byte) (*WebhookEvent, error)

	// ClientCredentials returns the credentials the mobile app needs to open the payment SDK
	// e.g. {"key_id": "rzp_..."} for Razorpay, {"mid": "...", "txn_token": "..."} for Paytm
	ClientCredentials() map[string]interface{}
}
