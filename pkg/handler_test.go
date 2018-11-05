package pkg

import (
	"testing"

	"github.com/wu8685/lpv-res-predicate/pkg/config"
)

func TestDuplicateHandler(t *testing.T) {
	tmpInit := func(*config.Config) (Handler, error) {
		return nil, nil
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expecting panic")
		}
	}()
	RegisterHandlerInit("exist", tmpInit)
	RegisterHandlerInit("exist", tmpInit)
}
