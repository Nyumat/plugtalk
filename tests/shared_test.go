package tests

import (
	"testing"

	"plugtalk/internal/shared"
)

func TestGenNick(t *testing.T) {
	nickname := shared.GenerateNickname()
	if nickname == "" {
		t.Errorf("expected non-empty, valid nickname; got %v", nickname)
	}
}
