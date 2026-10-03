package provider

import "os"

// ResolveAPIKey picks a key without inventing one.
// An explicit --api-key wins. Otherwise a provider-specific variable wins
// over TOOLPROBE_API_KEY, so an OpenAI key in TOOLPROBE_API_KEY is not sent
// to Anthropic or Gemini when those providers have their own variable set.
func ResolveAPIKey(providerName string, explicit string, explicitSet bool) string {
	if explicitSet {
		return explicit
	}
	switch providerName {
	case Anthropic:
		if v := firstEnv("TOOLPROBE_ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY"); v != "" {
			return v
		}
	case Gemini:
		if v := firstEnv("TOOLPROBE_GEMINI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"); v != "" {
			return v
		}
	case Cloudflare:
		if v := firstEnv("TOOLPROBE_CLOUDFLARE_API_TOKEN", "CLOUDFLARE_API_TOKEN"); v != "" {
			return v
		}
	}
	if v := os.Getenv("TOOLPROBE_API_KEY"); v != "" {
		return v
	}
	if providerName == OpenAI {
		return os.Getenv("OPENAI_API_KEY")
	}
	return ""
}

// ResolveAccountID returns the Cloudflare account id from the flag or env.
func ResolveAccountID(explicit string, explicitSet bool) string {
	if explicitSet {
		return explicit
	}
	if explicit != "" {
		return explicit
	}
	return firstEnv("TOOLPROBE_CLOUDFLARE_ACCOUNT_ID", "CLOUDFLARE_ACCOUNT_ID")
}

// ResolveModel applies the OpenAI default only for the OpenAI provider.
// Other providers require --model or TOOLPROBE_MODEL. Mock runs may omit it.
func ResolveModel(providerName, flagModel string, modelSet, mock bool) (string, error) {
	if modelSet {
		return flagModel, nil
	}
	if providerName == OpenAI {
		if flagModel != "" {
			return flagModel, nil
		}
		return "gpt-4o-mini", nil
	}
	if mock {
		return "", nil
	}
	return "", errf("--model is required for provider %s (toolprobe does not default a model id for this provider)", providerName)
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}
