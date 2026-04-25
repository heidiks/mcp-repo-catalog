package catalog

import "strings"

var domainKeywords = map[string][]string{
	"fiscal":    {"fiscal", "tax", "nfe", "nfce", "sefaz", "sped", "invoice", "imposto"},
	"identity":  {"auth", "login", "sso", "idp", "oauth", "gatekeeper", "dex", "scim"},
	"payments":  {"payment", "pagamento", "billing", "pix", "boleto", "checkout"},
	"logistics": {"shipping", "tracking", "logistics", "trip", "delivery", "carrier", "ciot"},
	"storage":   {"storage", "file", "upload", "blob", "document", "roz"},
	"messaging": {"webhook", "notification", "queue", "event", "consumer", "publisher"},
	"infra":     {"job", "executor", "cron", "pipeline", "deploy", "infra"},
	"security":  {"captcha", "certificate", "certificado", "signer", "crypto", "xml"},
}

func InferDomain(repoName, description, readme string) string {
	text := strings.ToLower(repoName + " " + description + " " + readme)

	bestDomain := ""
	bestScore := 0

	for domain, keywords := range domainKeywords {
		score := 0
		for _, kw := range keywords {
			if strings.Contains(text, kw) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestDomain = domain
		}
	}

	return bestDomain
}
