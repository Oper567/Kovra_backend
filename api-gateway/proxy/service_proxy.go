package proxy

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ServiceProxy is a lightweight reverse proxy for routing API Gateway
// requests to downstream microservices. It strips the gateway prefix
// and forwards the remaining path.
type ServiceProxy struct {
	targetBase string
	client     *http.Client
}

func NewServiceProxy(targetBase string) *ServiceProxy {
	return &ServiceProxy{
		targetBase: strings.TrimRight(targetBase, "/"),
		client: &http.Client{
			Timeout: 90 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Forward creates a Gin handler that proxies requests to the target service.
// The stripPrefix is removed from the request path before forwarding.
// Example: "/api/v1/wallet/balance" with stripPrefix="/api/v1/wallet"
// forwards to "{targetBase}/api/v1/wallet/balance"
func (p *ServiceProxy) Forward(stripPrefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Build target URL
		targetPath := c.Request.URL.Path
		if stripPrefix != "" {
			targetPath = strings.TrimPrefix(targetPath, stripPrefix)
			if targetPath == "" {
				targetPath = "/"
			}
		}

		targetURL, err := url.Parse(p.targetBase + stripPrefix + targetPath)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{
				"success": false,
				"error":   "Failed to route request",
				"code":    "PROXY_ERROR",
			})
			return
		}
		targetURL.RawQuery = c.Request.URL.RawQuery

		// Create proxy request
		proxyReq, err := http.NewRequestWithContext(
			c.Request.Context(),
			c.Request.Method,
			targetURL.String(),
			c.Request.Body,
		)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{
				"success": false,
				"error":   "Failed to create proxy request",
				"code":    "PROXY_ERROR",
			})
			return
		}

		// Copy headers (including X-User-ID injected by JWT middleware)
		for key, values := range c.Request.Header {
			for _, v := range values {
				proxyReq.Header.Add(key, v)
			}
		}
		proxyReq.Header.Set("X-Forwarded-For", c.ClientIP())
		proxyReq.Header.Set("X-Forwarded-Host", c.Request.Host)

		// Execute request
		resp, err := p.client.Do(proxyReq)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{
				"success": false,
				"error":   "Downstream service unavailable",
				"code":    "SERVICE_DOWN",
				"details": "Failed to reach: " + targetURL.String(),
			})
			return
		}
		defer resp.Body.Close()

		// Copy response headers
		for key, values := range resp.Header {
			for _, v := range values {
				c.Writer.Header().Add(key, v)
			}
		}

		c.Writer.WriteHeader(resp.StatusCode)
		io.Copy(c.Writer, resp.Body)
	}
}
