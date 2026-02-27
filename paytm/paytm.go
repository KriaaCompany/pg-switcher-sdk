// Package paytm implements the pg.PaymentGateway interface for Paytm AllInOne SDK.
// It calls Paytm's REST API (v1) to initiate transactions and verify payments
// using only Go standard library — no external Paytm SDK required.
package paytm

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	pg "github.com/KriaaCompany/pg-switcher-sdk"
)

const (
	productionBase = "https://securegw.paytm.in"
	stagingBase    = "https://securegw-stage.paytm.in"
)

// Config holds Paytm payment gateway credentials.
type Config struct {
	MID           string // Merchant ID
	MerchantKey   string
	Website       string // e.g. "WEBSTAGING" or "DEFAULT"
	CallbackURL   string
	WebhookSecret string
	Production    bool
}

// Adapter implements pg.PaymentGateway for Paytm.
type Adapter struct {
	cfg    Config
	client *http.Client
}

// New creates a new Paytm PaymentGateway adapter.
func New(cfg Config) *Adapter {
	return &Adapter{cfg: cfg, client: &http.Client{}}
}

// Name returns the gateway identifier.
func (a *Adapter) Name() string { return "paytm" }

// ─── CreateOrder ─────────────────────────────────────────────────────────────

// initiateBody is the inner "body" of the initiateTransaction request.
type initiateBody struct {
	RequestType string    `json:"requestType"`
	MID         string    `json:"mid"`
	WebsiteName string    `json:"websiteName"`
	OrderID     string    `json:"orderId"`
	TxnAmount   txnAmount `json:"txnAmount"`
	UserInfo    userInfo  `json:"userInfo"`
	CallbackURL string    `json:"callbackUrl,omitempty"`
}

type txnAmount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

type userInfo struct {
	CustID string `json:"custId"`
}

type initiateHead struct {
	Version   string `json:"version"`
	ChannelID string `json:"channelId"`
	TokenType string `json:"tokenType"`
	Signature string `json:"signature"`
}

type initiateRequest struct {
	Body initiateBody `json:"body"`
	Head initiateHead `json:"head"`
}

type initiateResponse struct {
	Head struct {
		Signature string `json:"signature"`
	} `json:"head"`
	Body struct {
		ResultInfo struct {
			ResultStatus string `json:"resultStatus"`
			ResultCode   string `json:"resultCode"`
			ResultMsg    string `json:"resultMsg"`
		} `json:"resultInfo"`
		TxnToken string `json:"txnToken"`
	} `json:"body"`
}

// CreateOrder calls Paytm's initiateTransaction API and returns the txn_token
// required by the mobile AllInOne SDK.
func (a *Adapter) CreateOrder(ctx context.Context, req pg.CreateOrderRequest) (*pg.CreateOrderResponse, error) {
	amountRupees := fmt.Sprintf("%.2f", float64(req.Amount)/100.0)
	website := a.cfg.Website
	if website == "" {
		if a.cfg.Production {
			website = "DEFAULT"
		} else {
			website = "WEBSTAGING"
		}
	}

	body := initiateBody{
		RequestType: "Payment",
		MID:         a.cfg.MID,
		WebsiteName: website,
		OrderID:     req.Receipt,
		TxnAmount:   txnAmount{Value: amountRupees, Currency: req.Currency},
		UserInfo:    userInfo{CustID: "anonymous"},
		CallbackURL: a.cfg.CallbackURL,
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("paytm: marshal body: %w", err)
	}

	payload := initiateRequest{
		Body: body,
		Head: initiateHead{
			Version:   "v1",
			ChannelID: "WAP",
			TokenType: "AES",
			Signature: computeSignature(string(bodyJSON), a.cfg.MerchantKey),
		},
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("paytm: marshal request: %w", err)
	}

	base := stagingBase
	if a.cfg.Production {
		base = productionBase
	}
	url := fmt.Sprintf("%s/theia/api/v1/initiateTransaction?mid=%s&orderId=%s", base, a.cfg.MID, req.Receipt)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadJSON))
	if err != nil {
		return nil, fmt.Errorf("paytm: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("paytm: HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("paytm: read response: %w", err)
	}

	var txnResp initiateResponse
	if err := json.Unmarshal(respBody, &txnResp); err != nil {
		return nil, fmt.Errorf("paytm: decode response: %w", err)
	}

	if txnResp.Body.ResultInfo.ResultStatus != "S" {
		return nil, fmt.Errorf("paytm: order creation failed: %s (code %s)",
			txnResp.Body.ResultInfo.ResultMsg, txnResp.Body.ResultInfo.ResultCode)
	}
	if txnResp.Body.TxnToken == "" {
		return nil, fmt.Errorf("paytm: empty txn_token in response")
	}

	return &pg.CreateOrderResponse{
		GatewayOrderID: req.Receipt, // Paytm uses our orderId as the identifier
		Amount:         req.Amount,
		Currency:       req.Currency,
		Extra: map[string]interface{}{
			"txn_token": txnResp.Body.TxnToken,
			"mid":       a.cfg.MID,
		},
	}, nil
}

// ─── VerifyPayment ───────────────────────────────────────────────────────────

// VerifyPayment confirms a Paytm payment by querying the order status API
// server-side (more reliable than client-checksum verification).
func (a *Adapter) VerifyPayment(ctx context.Context, req pg.VerifyPaymentRequest) (bool, error) {
	type statusBody struct {
		MID     string `json:"mid"`
		OrderID string `json:"orderId"`
	}

	body := statusBody{MID: a.cfg.MID, OrderID: req.GatewayOrderID}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return false, fmt.Errorf("paytm: marshal status body: %w", err)
	}

	payload := map[string]interface{}{
		"body": body,
		"head": map[string]interface{}{
			"signature": computeSignature(string(bodyJSON), a.cfg.MerchantKey),
			"tokenType": "AES",
		},
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("paytm: marshal status request: %w", err)
	}

	base := stagingBase
	if a.cfg.Production {
		base = productionBase
	}
	url := fmt.Sprintf("%s/v3/order/status", base)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadJSON))
	if err != nil {
		return false, fmt.Errorf("paytm: create status request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("paytm: order status HTTP request: %w", err)
	}
	defer resp.Body.Close()

	var statusResp struct {
		Body struct {
			ResultInfo struct {
				ResultStatus string `json:"resultStatus"`
			} `json:"resultInfo"`
			TxnStatus string `json:"txnStatus"`
		} `json:"body"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return false, fmt.Errorf("paytm: decode status response: %w", err)
	}

	return statusResp.Body.TxnStatus == "TXN_SUCCESS", nil
}

// ─── Other gateway methods ────────────────────────────────────────────────────

// GetPaymentStatus queries a Paytm order's current status.
func (a *Adapter) GetPaymentStatus(ctx context.Context, gatewayOrderID string) (*pg.PaymentStatus, error) {
	ok, err := a.VerifyPayment(ctx, pg.VerifyPaymentRequest{GatewayOrderID: gatewayOrderID})
	if err != nil {
		return nil, err
	}
	status := "TXN_FAILURE"
	if ok {
		status = "TXN_SUCCESS"
	}
	return &pg.PaymentStatus{
		GatewayOrderID: gatewayOrderID,
		Status:         status,
		Paid:           ok,
	}, nil
}

// InitiateRefund initiates a Paytm refund via the Refund API.
func (a *Adapter) InitiateRefund(_ context.Context, req pg.RefundRequest) (*pg.RefundResponse, error) {
	// Paytm refund requires TXNID (GatewayPaymentID) and REFUNDID
	// Full implementation follows Paytm's /refund/apply/v2 endpoint
	return nil, fmt.Errorf("paytm: refund not yet implemented")
}

// VerifyWebhookSignature verifies the X-Paytm-Signature header.
func (a *Adapter) VerifyWebhookSignature(payload []byte, headers map[string]string) bool {
	sig := headers["x-paytm-signature"]
	if sig == "" || a.cfg.WebhookSecret == "" {
		return false
	}
	expected := computeSignature(string(payload), a.cfg.WebhookSecret)
	return sig == expected
}

// ParseWebhookEvent parses a Paytm webhook payload into a normalised WebhookEvent.
func (a *Adapter) ParseWebhookEvent(payload []byte) (*pg.WebhookEvent, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("paytm: parse webhook: %w", err)
	}

	evt := &pg.WebhookEvent{Type: pg.WebhookEventUnknown, Raw: raw}

	// Paytm webhook body contains a "body" key with transaction details
	if body, ok := raw["body"].(map[string]interface{}); ok {
		if status, ok := body["txnStatus"].(string); ok {
			switch status {
			case "TXN_SUCCESS":
				evt.Type = pg.WebhookEventPaymentSuccess
			case "TXN_FAILURE":
				evt.Type = pg.WebhookEventPaymentFailed
			}
		}
		if ordID, ok := body["orderId"].(string); ok {
			evt.GatewayOrderID = ordID
		}
		if txnID, ok := body["txnId"].(string); ok {
			evt.GatewayPaymentID = txnID
		}
	}

	return evt, nil
}

// ClientCredentials returns the Paytm credentials the mobile SDK needs.
// The txn_token is not here — it comes from CreateOrderResponse.Extra and is
// merged into client_payload by the payment usecase.
func (a *Adapter) ClientCredentials() map[string]interface{} {
	return map[string]interface{}{
		"mid":        a.cfg.MID,
		"production": a.cfg.Production,
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// computeSignature computes HMAC-SHA256 of data using key, base64-encoded.
func computeSignature(data, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
