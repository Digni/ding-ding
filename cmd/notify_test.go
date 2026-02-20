package cmd

import "testing"

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
