package pg

// RazorpayConfig holds Razorpay payment gateway credentials
type RazorpayConfig struct {
	KeyID         string
	KeySecret     string
	WebhookSecret string
}

// RazorpayXConfig holds RazorpayX payout gateway credentials
type RazorpayXConfig struct {
	KeyID         string
	KeySecret     string
	AccountNumber string
	WebhookSecret string
}

// PaytmConfig holds Paytm payment gateway credentials
type PaytmConfig struct {
	MID         string // Merchant ID
	MerchantKey string
	Website     string // e.g. "WEBSTAGING" or "DEFAULT"
	CallbackURL string
	WebhookSecret string
	Production  bool
}

// Config aggregates all gateway credentials and selects which gateway to use
type Config struct {
	// Gateway selection (used by NewPayment / NewPayout static constructors)
	PaymentGateway string // "razorpay" | "paytm"
	PayoutGateway  string // "razorpayx" | "paytm" | "manual"

	Razorpay  RazorpayConfig
	RazorpayX RazorpayXConfig
	Paytm     PaytmConfig
}
