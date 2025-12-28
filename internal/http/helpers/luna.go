package helpers

import (
	"fmt"
)

// "1 2 3 4"

func CheckNumber(n string) (bool, error) {
	runes := []rune(n)
	var sum int
	double := false

	for i := len(runes) - 1; i >= 0; i-- {
		if runes[i] < '0' || runes[i] > '9' {
			return false, fmt.Errorf("incorrect caharacter in an order number %c", runes[i])
		}
		digit := int(runes[i] - '0')
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		double = !double
	}

	if sum%10 != 0 {
		return false, fmt.Errorf("incorrect order number, check luna failed")
	} else {
		return true, nil
	}
}
