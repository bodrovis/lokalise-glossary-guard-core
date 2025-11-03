package non_empty_file

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bodrovis/lokalise-glossary-guard-core/pkg/checks"
)

// These tests cover only the validation logic — no auto-fix involved.
func TestValidateNotEmpty(t *testing.T) {
	t.Run("non-empty file passes", func(t *testing.T) {
		res := validateNotEmpty(context.Background(), checks.Artifact{
			Data: []byte("term;description\nhello;world"),
		})
		if !res.OK {
			t.Fatalf("expected OK=true, got %+v", res)
		}
	})

	t.Run("empty file fails", func(t *testing.T) {
		res := validateNotEmpty(context.Background(), checks.Artifact{
			Data: []byte(""),
		})
		if res.OK {
			t.Fatalf("expected OK=false for empty file")
		}
		if !strings.Contains(res.Msg, "empty file") {
			t.Fatalf("expected message mentioning 'empty file', got %q", res.Msg)
		}
	})

	t.Run("whitespace-only file fails", func(t *testing.T) {
		res := validateNotEmpty(context.Background(), checks.Artifact{
			Data: []byte("   \n\t  "),
		})
		if res.OK {
			t.Fatalf("expected OK=false for whitespace-only file")
		}
		if !strings.Contains(res.Msg, "empty file") {
			t.Fatalf("expected message mentioning 'empty file', got %q", res.Msg)
		}
	})

	t.Run("cancelled context returns error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		res := validateNotEmpty(ctx, checks.Artifact{
			Data: []byte("term;description"),
		})
		if res.OK {
			t.Fatalf("expected OK=false for cancelled context")
		}
		if res.Err == nil {
			t.Fatalf("expected Err to be set for cancelled context")
		}
		if !strings.Contains(res.Msg, "validation cancelled") {
			t.Fatalf("expected message mentioning 'validation cancelled', got %q", res.Msg)
		}
	})

	t.Run("timeout context also returns error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()
		time.Sleep(time.Millisecond) // ensure timeout triggers

		res := validateNotEmpty(ctx, checks.Artifact{
			Data: []byte("term;description"),
		})
		if res.OK {
			t.Fatalf("expected OK=false for timed-out context")
		}
		if res.Err == nil {
			t.Fatalf("expected Err to be set for timed-out context")
		}
	})
}
