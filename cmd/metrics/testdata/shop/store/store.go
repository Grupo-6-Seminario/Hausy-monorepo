package store

type Repository interface {
	Get(id ID) (Item, error)
}

type Clock interface {
	Now() int
}

type Item struct {
	ID    ID
	Price int
}

type ID string

// Discount takes the eight paths of the worked example in the metrics test.
func Discount(price int, member bool, coupons []string) int {
	if price <= 0 || len(coupons) > 3 {
		return 0
	}
	total := price
	for _, c := range coupons {
		switch c {
		case "half":
			total /= 2
		case "tenoff", "ten":
			total -= 10
		default:
		}
	}
	if member && total > 100 {
		total -= 5
	}
	return total
}

func (i *Item) Cost(quantity int) int {
	if quantity > 10 {
		return i.Price * quantity * 9 / 10
	}
	return i.Price * quantity
}
