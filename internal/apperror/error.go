package apperror

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

var (
	ErrInvalidCurrency = &Error{"invalid_currency", "use two different supported currency codes"}
	ErrInvalidID       = &Error{"invalid_id", "update ID must be a UUID"}
	ErrInvalidKey      = &Error{"invalid_idempotency_key", "use a single key of 1 to 255 visible ASCII characters"}
	ErrKeyConflict     = &Error{"idempotency_conflict", "the idempotency key already belongs to a different currency pair"}
	ErrUpdateNotFound  = &Error{"update_not_found", "update not found"}
	ErrQuoteNotFound   = &Error{"quote_not_found", "no successful quote is available for this currency pair"}
)
