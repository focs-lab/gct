package instrumenter

import "testing"

func TestRequiredVersion(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want string
	}{
		{
			name: "default release version",
			want: defaultRequiredVersion,
		},
		{
			name: "local replace placeholder",
			opts: Options{ReplaceRoot: "/tmp/gct"},
			want: localReplaceVersion,
		},
		{
			name: "explicit version overrides local replace",
			opts: Options{ReplaceRoot: "/tmp/gct", Version: "v0.2.0"},
			want: "v0.2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := requiredVersion(tt.opts); got != tt.want {
				t.Fatalf("requiredVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
