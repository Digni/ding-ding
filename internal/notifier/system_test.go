package notifier

import "testing"

func TestXmlEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "ampersand",
			input: "&",
			want:  "&amp;",
		},
		{
			name:  "less than",
			input: "<",
			want:  "&lt;",
		},
		{
			name:  "greater than",
			input: ">",
			want:  "&gt;",
		},
		{
			name:  "double quote",
			input: `"`,
			want:  "&quot;",
		},
		{
			name:  "single quote",
			input: "'",
			want:  "&apos;",
		},
		{
			name:  "all special chars",
			input: `<>&"'`,
			want:  "&lt;&gt;&amp;&quot;&apos;",
		},
		{
			name:  "no special chars",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "mixed",
			input: `say "hello" & <bye>`,
			want:  "say &quot;hello&quot; &amp; &lt;bye&gt;",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := xmlEscape(tt.input)
			if got != tt.want {
				t.Errorf("xmlEscape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
