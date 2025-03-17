package core

// AdjustStock returns the quantity a stock level should hold after delta is
// applied. A rejected adjustment writes nothing because there is nothing to
// write: the caller persists the returned value or nothing at all.
func AdjustStock(onHand, delta int64) (int64, error) {
	if delta == 0 {
		return 0, Invalid("delta must not be zero")
	}
	result := onHand + delta
	if result < 0 {
		return 0, InsufficientStock("not enough stock on hand")
	}
	return result, nil
}

func CheckPartQuantity(quantity int64) error {
	if quantity < 1 {
		return Invalid("quantity must be greater than zero")
	}
	return nil
}

// ConsumeStock takes quantity units for a job part.
func ConsumeStock(onHand, quantity int64) (int64, error) {
	if err := CheckPartQuantity(quantity); err != nil {
		return 0, err
	}
	return AdjustStock(onHand, -quantity)
}

// ReleaseStock puts quantity units back when a job part is removed.
func ReleaseStock(onHand, quantity int64) (int64, error) {
	if err := CheckPartQuantity(quantity); err != nil {
		return 0, err
	}
	return AdjustStock(onHand, quantity)
}
