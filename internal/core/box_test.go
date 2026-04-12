package core

import (
	"context"
	"testing"
)

func TestBox(t *testing.T) {
	box := New(context.Background())
	err := box.Start("香港(多线)", false)
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = box.Stop() }()
	select {}
}
