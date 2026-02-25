// Package razorpay implements the pg.PaymentGateway interface using the Razorpay SDK.
package razorpay

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	rzp "github.com/razorpay/razorpay-go"

	pg "github.com/KriaaCompany/pg-switcher-sdk"
)

// Config holds Razorpay payment credentials
type Config struct {
	KeyID         string
	KeySecret     string
	WebhookSecret string
}

// Adapter wraps the Razorpay SDK and implements pg.PaymentGateway
type Adapter struct {
	cfg    Config
	client *rzp.Client
}

// New creates a new Razorpay PaymentGateway adapter
func New(cfg Config) *Adapter {
	return &Adapter{
		cfg:    cfg,
		client: rzp.NewClient(cfg.KeyID, cfg.KeySecret),
	}
}

// Name returns the gateway identifier
func (a *Adapter) Name() string { return "razorpay" }

// CreateOrder creates a Razorpay order
func (a *Adapter) CreateOrder(_ context.Context, req pg.CreateOrderRequest) (*pg.CreateOrderResponse, error) {
	notes := make(map[string]interface{})
	for k, v := range req.Notes {
		notes[k] = v
	}

	body := map[string]interface{}{
		"amount":   req.Amount,
		"currency": req.Currency,
		"receipt":  req.Receipt,
		"notes":    notes,
	}

	result, err := a.client.Order.Create(body, nil)
	if err != nil {
		return nil, fmt.Errorf("razorpay: create order failed: %w", err)
	}

	id, _ := result["id"].(string)
	return &pg.CreateOrderResponse{
		GatewayOrderID: id,
		Amount:         req.Amount,
		Currency:       req.Currency,
	}, nil
}

// VerifyPayment verifies the Razorpay payment signature
func (a *Adapter) VerifyPayment(_ context.Context, req pg.VerifyPaymentRequest) (bool, error) {
	data := req.GatewayOrderID + "|" + req.GatewayPaymentID
	h := hmac.New(sha256.New, []byte(a.cfg.KeySecret))
	h.Write([]byte(data))
	expected := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(req.Signature), []byte(expected)), nil
}

// GetPaymentStatus queries a Razorpay order's status
func (a *Adapter) GetPaymentStatus(_ context.Context, gatewayOrderID string) (*pg.PaymentStatus, error) {
	result, err := a.client.Order.Fetch(gatewayOrderID, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("razorpay: fetch order failed: %w", err)
	}
	status, _ := result["status"].(string)
	return &pg.PaymentStatus{
		GatewayOrderID: gatewayOrderID,
		Status:         status,
		Paid:           status == "paid",
	}, nil
}

// InitiateRefund creates a Razorpay refund
func (a *Adapter) InitiateRefund(_ context.Context, req pg.RefundRequest) (*pg.RefundResponse, error) {
	notes := make(map[string]interface{})
	for k, v := range req.Notes {
		notes[k] = v
	}
	body := map[string]interface{}{
		"amount": req.Amount,
		"notes":  notes,
	}
	result, err := a.client.Payment.Refund(req.GatewayPaymentID, int(req.Amount), body, nil)
	if err != nil {
		return nil, fmt.Errorf("razorpay: refund failed: %w", err)
	}
	id, _ := result["id"].(string)
	status, _ := result["status"].(string)
	return &pg.RefundResponse{RefundID: id, Amount: req.Amount, Status: status}, nil
}

// VerifyWebhookSignature verifies the X-Razorpay-Signature header
func (a *Adapter) VerifyWebhookSignature(payload []byte, headers map[string]string) bool {
	sig := headers["x-razorpay-signature"]
	if sig == "" {
		return false
	}
	h := hmac.New(sha256.New, []byte(a.cfg.WebhookSecret))
	h.Write(payload)
	expected := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}

// ParseWebhookEvent parses a Razorpay webhook payload into a pg.WebhookEvent
func (a *Adapter) ParseWebhookEvent(payload []byte) (*pg.WebhookEvent, error) {
	var envelope struct {
		Event   string                 `json:"event"`
		Payload map[string]interface{} `json:"payload"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("razorpay: failed to parse webhook: %w", err)
	}

	evt := &pg.WebhookEvent{Raw: envelope.Payload}

	// Extract common fields from nested payload
	paymentEntity := extractEntity(envelope.Payload, "payment")
	orderEntity := extractEntity(envelope.Payload, "order")
	refundEntity := extractEntity(envelope.Payload, "refund")
	disputeEntity := extractEntity(envelope.Payload, "dispute")

	if v, ok := paymentEntity["order_id"].(string); ok {
		evt.GatewayOrderID = v
	}
	if v, ok := orderEntity["id"].(string); ok && evt.GatewayOrderID == "" {
		evt.GatewayOrderID = v
	}
	if v, ok := paymentEntity["id"].(string); ok {
		evt.GatewayPaymentID = v
	}
	if v, ok := refundEntity["id"].(string); ok {
		evt.RefundID = v
	}
	if v, ok := disputeEntity["id"].(string); ok {
		evt.DisputeID = v
	}
	if v, ok := paymentEntity["error_description"].(string); ok {
		evt.FailureReason = v
	}

	switch {
	case envelope.Event == "payment.captured":
		evt.Type = pg.WebhookEventPaymentSuccess
	case envelope.Event == "payment.failed":
		evt.Type = pg.WebhookEventPaymentFailed
	case envelope.Event == "order.paid":
		evt.Type = pg.WebhookEventOrderPaid
		// For order.paid, also get payment ID from nested payment entity
		if v, ok := paymentEntity["id"].(string); ok {
			evt.GatewayPaymentID = v
		}
	case envelope.Event == "refund.processed":
		evt.Type = pg.WebhookEventRefundSuccess
		if v, ok := refundEntity["payment_id"].(string); ok {
			evt.GatewayPaymentID = v
		}
	case envelope.Event == "refund.failed":
		evt.Type = pg.WebhookEventRefundFailed
		if v, ok := refundEntity["payment_id"].(string); ok {
			evt.GatewayPaymentID = v
		}
	case strings.HasPrefix(envelope.Event, "payment.dispute."):
		switch envelope.Event {
		case "payment.dispute.created":
			evt.Type = pg.WebhookEventDisputeCreated
		case "payment.dispute.won":
			evt.Type = pg.WebhookEventDisputeWon
		case "payment.dispute.lost":
			evt.Type = pg.WebhookEventDisputeLost
		case "payment.dispute.closed":
			evt.Type = pg.WebhookEventDisputeClosed
		default:
			evt.Type = pg.WebhookEventUnknown
		}
		if v, ok := disputeEntity["payment_id"].(string); ok {
			evt.GatewayPaymentID = v
		}
		if v, ok := disputeEntity["amount"].(float64); ok {
			evt.Amount = int64(v)
		}
		if v, ok := disputeEntity["currency"].(string); ok {
			evt.Currency = v
		}
	default:
		evt.Type = pg.WebhookEventUnknown
	}

	return evt, nil
}

// ClientCredentials returns the Razorpay key_id for the mobile SDK
func (a *Adapter) ClientCredentials() map[string]interface{} {
	return map[string]interface{}{
		"key_id": a.cfg.KeyID,
	}
}

// extractEntity safely extracts "entity" from a nested payload object
func extractEntity(payload map[string]interface{}, key string) map[string]interface{} {
	obj, ok := payload[key].(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}
	entity, ok := obj["entity"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}
	return entity
}
