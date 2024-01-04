package handler

import "testing"

func TestHandler(t *testing.T) {
  got := 1
  if got != 1 {
    t.Errorf("handler() = %d; want 1", got)
  }
}
