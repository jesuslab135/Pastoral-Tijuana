package config

import "testing"

func TestPublicHost(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{raw: "https://calendario.parroquia.mx", want: "calendario.parroquia.mx"},
		{raw: "http://localhost:8080", want: "localhost"},
		// No scheme: url.Parse accepts this without error but finds no host,
		// which must not silently fall back to some other domain.
		{raw: "calendario.parroquia.mx", wantErr: true},
		{raw: "", wantErr: true},
		{raw: "://nope", wantErr: true},
	} {
		got, err := Config{PublicBaseURL: tc.raw}.PublicHost()
		if tc.wantErr {
			if err == nil {
				t.Errorf("PublicHost(%q) = %q, want an error", tc.raw, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("PublicHost(%q): unexpected error: %v", tc.raw, err)
			continue
		}
		if got != tc.want {
			t.Errorf("PublicHost(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
