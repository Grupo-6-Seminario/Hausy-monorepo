package logging_test

import (
	"fmt"
	"syscall"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/logging"
)

func TestErrorClassNamesConnectionRefusalWithoutErrorText(t *testing.T) {
	err := fmt.Errorf("postgresql://user:secret@localhost:5432/hausy: %w", syscall.ECONNREFUSED)
	if got := logging.ErrorClass(err); got != "connection_refused" {
		t.Fatalf("got %q, want connection_refused", got)
	}
}
