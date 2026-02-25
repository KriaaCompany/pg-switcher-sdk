package pg

import "context"

// PayoutWebhookEventType represents a payout webhook event type
type PayoutWebhookEventType string

const (
	PayoutWebhookEventProcessed PayoutWebhookEventType = "payout.processed"
	PayoutWebhookEventFailed    PayoutWebhookEventType = "payout.failed"
	PayoutWebhookEventReversed  PayoutWebhookEventType = "payout.reversed"
	PayoutWebhookEventUnknown   PayoutWebhookEventType = "unknown"
)

// CreateContactRequest contains fields for creating a payout contact
type CreateContactRequest struct {
	Name        string
	Email       string
	Phone       string
	ReferenceID string // internal user ID
}

// ContactResponse is returned after creating or updating a contact
type ContactResponse struct {
	ContactID string
}

// CreateFundAccountRequest contains fields for creating a fund account
type CreateFundAccountRequest struct {
	ContactID     string
	AccountType   string // "vpa" or "bank_account"
	VPA           string // UPI VPA (when AccountType == "vpa")
	AccountName   string // bank account holder name
	AccountNumber string
	IFSC          string
}

// FundAccountResponse is returned after creating a fund account
type FundAccountResponse struct {
	FundAccountID string
}

// InitiatePayoutRequest contains fields for initiating a payout
type InitiatePayoutRequest struct {
	FundAccountID string
	Amount        int64  // in paise
	Currency      string // "INR"
	Mode          string // "UPI", "NEFT", "IMPS", "RTGS"
	ReferenceID   string // idempotency key (internal payout UUID)
	Narration     string
}

// PayoutResponse is returned after initiating a payout
type PayoutResponse struct {
	GatewayPayoutID string
	Status          string // "processing", "pending_manual", etc.
}

// PayoutStatusResponse is returned when querying a payout's status
type PayoutStatusResponse struct {
	GatewayPayoutID string
	Status          string
	FailureReason   string
}

// PayoutWebhookEvent is a normalised payout webhook event
type PayoutWebhookEvent struct {
	Type            PayoutWebhookEventType
	GatewayPayoutID string
	FailureReason   string
	// Raw contains the original parsed payload
	Raw map[string]interface{}
}

// PayoutGateway is the common interface that all payout gateway adapters implement
type PayoutGateway interface {
	// Name returns the unique gateway identifier (e.g. "razorpayx", "paytm", "manual")
	Name() string

	// CreateContact creates a payout contact (organiser onboarding)
	CreateContact(ctx context.Context, req CreateContactRequest) (*ContactResponse, error)

	// UpdateContact updates an existing payout contact
	UpdateContact(ctx context.Context, contactID string, req CreateContactRequest) (*ContactResponse, error)

	// CreateFundAccount creates a fund account tied to a contact
	CreateFundAccount(ctx context.Context, req CreateFundAccountRequest) (*FundAccountResponse, error)

	// InitiatePayout sends a payout to the organiser's fund account
	InitiatePayout(ctx context.Context, req InitiatePayoutRequest) (*PayoutResponse, error)

	// GetPayoutStatus queries the current status of a payout
	GetPayoutStatus(ctx context.Context, gatewayPayoutID string) (*PayoutStatusResponse, error)

	// VerifyWebhookSignature returns true if the webhook payload is authentic
	VerifyWebhookSignature(payload []byte, headers map[string]string) bool

	// ParseWebhookEvent parses a raw payout webhook payload into a normalised event
	ParseWebhookEvent(payload []byte) (*PayoutWebhookEvent, error)

	// IsManual returns true for the manual payout adapter (no API calls)
	IsManual() bool
}
