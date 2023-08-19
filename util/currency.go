package util

// Constants for all supported currency
const (
	USD = "USD"
	EUR = "EUR"
	CAD = "CAD"
)

// isSupportedCurrency returns true if the currency is supported
func IsSupportedCurrency(currency string) bool {
	switch currency {
	case USD, EUR, CAD:
		return true
	}
	return false
}
