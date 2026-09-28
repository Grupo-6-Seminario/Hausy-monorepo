package main

import (
	"fmt"

	"example.com/audit"
	"example.com/shop/pricing"
	"example.com/shop/store"
)

var _ store.Clock

func main() {
	quote, err := pricing.Price(nil, "a")
	if err != nil {
		audit.Log(err.Error())
		return
	}
	fmt.Println(quote.Total)
}
