# pg-switcher-sdk

A Go SDK that provides a unified interface for multiple payment and payout gateways, with hot-swappable gateway selection at request time.

## Installation

```bash
go get github.com/KriaaCompany/pg-switcher-sdk@v1.0.0
```

## Overview

The SDK defines two core interfaces — `PaymentGateway` and `PayoutGateway` — and provides a dynamic switcher that resolves which gateway to use on every request via a `GatewayResolver` function. This allows gateway configuration changes (e.g. switching from Razorpay to Paytm) to take effect immediately without a server restart.

## Interfaces

### PaymentGateway

Implemented by all payment collection adapters.

```go
type PaymentGateway interface {
    Name() string
    CreateOrder(ctx context.Context, req CreateOrderRequest) (*CreateOrderResponse, error)
    VerifyPayment(ctx context.Context, req VerifyPaymentRequest) (bool, error)
    GetPaymentStatus(ctx context.Context, gatewayOrderID string) (*PaymentStatus, error)
    InitiateRefund(ctx context.Context, req RefundRequest) (*RefundResponse, error)
    VerifyWebhookSignature(payload []byte, headers map[string]string) bool
    ParseWebhookEvent(payload []byte) (*WebhookEvent, error)
    ClientCredentials() map[string]interface{}
}
```

### PayoutGateway

Implemented by all payout/disbursement adapters.

```go
type PayoutGateway interface {
    Name() string
    CreateContact(ctx context.Context, req CreateContactRequest) (*ContactResponse, error)
    UpdateContact(ctx context.Context, contactID string, req CreateContactRequest) (*ContactResponse, error)
    CreateFundAccount(ctx context.Context, req CreateFundAccountRequest) (*FundAccountResponse, error)
    InitiatePayout(ctx context.Context, req InitiatePayoutRequest) (*PayoutResponse, error)
    GetPayoutStatus(ctx context.Context, gatewayPayoutID string) (*PayoutStatusResponse, error)
    VerifyWebhookSignature(payload []byte, headers map[string]string) bool
    ParseWebhookEvent(payload []byte) (*PayoutWebhookEvent, error)
    IsManual() bool
}
```

## Available Adapters

| Package | Gateway | Interface |
|---------|---------|-----------|
| `razorpay/` | Razorpay | `PaymentGateway` |
| `paytm/` | Paytm | `PaymentGateway` |
| `razorpayx/` | RazorpayX | `PayoutGateway` |
| `paytm_payout/` | Paytm Payouts | `PayoutGateway` |
| `manual/` | Manual (no API) | `PayoutGateway` |

The **manual** payout adapter is used when payouts are processed offline. It does not make any API calls and returns `IsManual() == true`, signalling the caller to handle transfer confirmation manually.

## Dynamic Switcher

The `DynamicPaymentSwitcher` and `DynamicPayoutSwitcher` wrap a map of registered adapters and a `GatewayResolver` function. On every call they resolve the active gateway by name and delegate to it.

```go
type GatewayResolver func(ctx context.Context) (string, error)
```

### Payment Switcher

```go
import (
    pg "github.com/KriaaCompany/pg-switcher-sdk"
    "github.com/KriaaCompany/pg-switcher-sdk/razorpay"
    "github.com/KriaaCompany/pg-switcher-sdk/paytm"
)

resolver := func(ctx context.Context) (string, error) {
    // Return the active gateway name from your config/database
    return adminConfigRepo.GetPaymentGateway(ctx)
}

switcher := pg.NewDynamicPaymentSwitcher(
    map[string]pg.PaymentGateway{
        "razorpay": razorpay.New(pg.RazorpayConfig{
            KeyID:         os.Getenv("RAZORPAY_KEY_ID"),
            KeySecret:     os.Getenv("RAZORPAY_KEY_SECRET"),
            WebhookSecret: os.Getenv("RAZORPAY_WEBHOOK_SECRET"),
        }),
        "paytm": paytm.New(pg.PaytmConfig{...}),
    },
    resolver,
)

// Use switcher as a PaymentGateway — gateway resolved per request
order, err := switcher.CreateOrder(ctx, pg.CreateOrderRequest{
    Amount:   10000, // in paise
    Currency: "INR",
    Receipt:  "booking_ref_123",
})
```

### Payout Switcher

```go
import (
    pg "github.com/KriaaCompany/pg-switcher-sdk"
    "github.com/KriaaCompany/pg-switcher-sdk/razorpayx"
    "github.com/KriaaCompany/pg-switcher-sdk/manual"
)

payoutSwitcher := pg.NewDynamicPayoutSwitcher(
    map[string]pg.PayoutGateway{
        "razorpayx": razorpayx.New(pg.RazorpayXConfig{
            KeyID:         os.Getenv("RAZORPAYX_KEY_ID"),
            KeySecret:     os.Getenv("RAZORPAYX_KEY_SECRET"),
            AccountNumber: os.Getenv("RAZORPAYX_ACCOUNT_NUMBER"),
            WebhookSecret: os.Getenv("RAZORPAYX_WEBHOOK_SECRET"),
        }),
        "manual": manual.New(),
    },
    resolver,
)

resp, err := payoutSwitcher.InitiatePayout(ctx, pg.InitiatePayoutRequest{
    FundAccountID: "fa_xxx",
    Amount:        50000, // in paise
    Currency:      "INR",
    Mode:          "UPI",
    ReferenceID:   "payout_uuid_here",
    Narration:     "Event payout",
})
```

### Webhook Handling

For webhook signature verification, the dynamic switcher tries all registered adapters (since the incoming request doesn't carry gateway context):

```go
// Verifies against all registered gateways; returns true if any match
valid := switcher.VerifyWebhookSignature(payload, headers)

// Parses using the currently active gateway (falls back to trying all)
event, err := switcher.ParseWebhookEvent(payload)
```

### Active Gateway Name

```go
name, err := switcher.ActiveGatewayName(ctx)
// e.g. "razorpay", "paytm"
```

## Configuration

```go
type RazorpayConfig struct {
    KeyID         string
    KeySecret     string
    WebhookSecret string
}

type RazorpayXConfig struct {
    KeyID         string
    KeySecret     string
    AccountNumber string
    WebhookSecret string
}

type PaytmConfig struct {
    MID           string // Merchant ID
    MerchantKey   string
    Website       string // e.g. "WEBSTAGING" or "DEFAULT"
    CallbackURL   string
    WebhookSecret string
    Production    bool
}
```

## Webhook Event Types

### Payment

| Constant | Value |
|----------|-------|
| `WebhookEventPaymentSuccess` | `payment.success` |
| `WebhookEventPaymentFailed` | `payment.failed` |
| `WebhookEventOrderPaid` | `order.paid` |
| `WebhookEventRefundSuccess` | `refund.success` |
| `WebhookEventRefundFailed` | `refund.failed` |
| `WebhookEventDisputeCreated` | `dispute.created` |
| `WebhookEventDisputeWon` | `dispute.won` |
| `WebhookEventDisputeLost` | `dispute.lost` |
| `WebhookEventDisputeClosed` | `dispute.closed` |

### Payout

| Constant | Value |
|----------|-------|
| `PayoutWebhookEventProcessed` | `payout.processed` |
| `PayoutWebhookEventFailed` | `payout.failed` |
| `PayoutWebhookEventReversed` | `payout.reversed` |

## Requirements

- Go 1.21+
