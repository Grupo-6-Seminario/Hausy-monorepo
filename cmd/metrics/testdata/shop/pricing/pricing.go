package pricing

import (
	"fmt"

	"example.com/shop/store"
)

type Quote struct{ Total int }

func Price(repository store.Repository, id store.ID) (Quote, error) {
	item, err := repository.Get(id)
	if err != nil {
		return Quote{}, fmt.Errorf("price %s: %w", id, err)
	}
	return Quote{Total: store.Discount(item.Price, false, nil)}, nil
}
