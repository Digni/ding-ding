package cmd

import (
	"errors"
	"testing"
)

func TestHasMistypedTestLocalArg(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want bool
	}{
		{
			name: "correct long flag",
			argv: []string{"notify", "--test-local", "-m", "hi"},
			want: false,
		},
		{
			name: "mistyped single dash",
			argv: []string{"notify", "-test-local", "-m", "hi"},
			want: true,
		},
		{
			name: "message value looks like flag",
			argv: []string{"notify", "-m", "-test-local"},
			want: false,
		},
		{
			name: "mistyped with equals",
			argv: []string{"notify", "-test-local=true", "-m", "hi"},
			want: true,
		},
		{
			name: "positional after double dash",
			argv: []string{"notify", "--", "-test-local"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasMistypedTestLocalArg(tt.argv)
			if got != tt.want {
				t.Fatalf("hasMistypedTestLocalArg(%v) = %v, want %v", tt.argv, got, tt.want)
			}
		})
	}
}

func TestIsBestEffortNotifyError(t *testing.T) {
	if isBestEffortNotifyError(nil) {
		t.Fatal("expected nil error to be non-best-effort")
	}

	deliveryErr := &notifyDeliveryError{err: errors.New("push backend unavailable")}
	if !isBestEffortNotifyError(deliveryErr) {
		t.Fatal("expected notifyDeliveryError to be best-effort")
	}

	wrapped := errors.New("wrap: " + deliveryErr.Error())
	if isBestEffortNotifyError(wrapped) {
		t.Fatal("expected plain wrapped string error to be non-best-effort")
	}

	wrappedTyped := errors.Join(errors.New("prefix"), deliveryErr)
	if !isBestEffortNotifyError(wrappedTyped) {
		t.Fatal("expected joined typed delivery error to be best-effort")
	}
}
