package catalog

import "testing"

func TestInferDomain(t *testing.T) {
	tests := []struct {
		name     string
		repo     string
		desc     string
		readme   string
		expected string
	}{
		{"fiscal from name", "core-fiscal", "", "", "fiscal"},
		{"fiscal from desc", "some-repo", "handles NFe emission", "", "fiscal"},
		{"identity from name", "auth-gatekeeper", "", "", "identity"},
		{"identity from readme", "my-service", "", "SSO integration with Dex IDP", "identity"},
		{"payments", "pagamentos", "payment processing", "", "payments"},
		{"logistics", "shopee-trip-status", "tracking shipments", "", "logistics"},
		{"storage", "roz-storage", "", "", "storage"},
		{"security", "certificado-digital", "XML signer", "", "security"},
		{"messaging", "webhook", "event consumer", "", "messaging"},
		{"infra", "job-executor", "cron jobs", "", "infra"},
		{"no match", "random-app", "does stuff", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferDomain(tt.repo, tt.desc, tt.readme)
			if got != tt.expected {
				t.Errorf("InferDomain(%q, %q, %q) = %q, want %q",
					tt.repo, tt.desc, tt.readme, got, tt.expected)
			}
		})
	}
}
