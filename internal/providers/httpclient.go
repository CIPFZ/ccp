package providers

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

func NewHTTPClient(proxy ProxyConfig) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy.Enabled {
		proxyURL, err := url.Parse(proxy.URL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy url %q: %w", proxy.URL, err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	} else {
		transport.Proxy = nil
	}
	return &http.Client{
		Timeout:   10 * time.Minute,
		Transport: transport,
	}, nil
}
