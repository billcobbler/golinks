package redirect

import (
	"testing"
)

func TestSubstitutePattern(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		remaining string
		want      string
	}{
		{
			name:      "wildcard substitution",
			template:  "https://github.com/{*}",
			remaining: "myorg/myrepo",
			want:      "https://github.com/myorg/myrepo",
		},
		{
			name:      "numbered substitution",
			template:  "https://jira.example.com/browse/{1}",
			remaining: "PROJ-123",
			want:      "https://jira.example.com/browse/PROJ-123",
		},
		{
			name:      "multiple numbered params",
			template:  "https://example.com/{1}/issues/{2}",
			remaining: "myrepo/42",
			want:      "https://example.com/myrepo/issues/42",
		},
		{
			name:      "both wildcard and numbered",
			template:  "https://example.com/{1}?q={*}",
			remaining: "search/term",
			want:      "https://example.com/search?q=search/term",
		},
		{
			name:      "no placeholder",
			template:  "https://example.com/static",
			remaining: "ignored",
			want:      "https://example.com/static",
		},
		{
			name:      "empty remaining",
			template:  "https://example.com/{*}",
			remaining: "",
			want:      "https://example.com/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substitutePattern(tt.template, tt.remaining)
			if got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}
