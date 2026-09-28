package store_test

import (
	"testing"

	"example.com/shop/pricing"
	"example.com/shop/store"
)

func TestImportsOfTestFilesAreNotCoupling(t *testing.T) {
	if (pricing.Quote{}).Total != 0 {
		t.Fatal("unreachable")
	}
}

func BenchmarkDiscount(b *testing.B) {
	for b.Loop() {
		store.Discount(200, true, []string{"half", "ten"})
	}
}
