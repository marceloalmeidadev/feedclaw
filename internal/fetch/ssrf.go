package fetch

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// SecurityMode controls how outbound targets are vetted. There is deliberately
// no "loose" mode: the engine never disables the SSRF guard.
type SecurityMode string

const (
	// ModeRestricted is the default: any public http/https host is allowed but
	// private, loopback and link-local destinations are blocked.
	ModeRestricted SecurityMode = "restricted"
	// ModeAllowlist additionally requires the host to match AllowedHosts.
	ModeAllowlist SecurityMode = "allowlist"
)

// Guard vets outbound requests to prevent SSRF against internal services.
type Guard struct {
	Mode         SecurityMode
	AllowedHosts []string // suffix match, e.g. "example.com" allows sub.example.com
	resolver     *net.Resolver
}

// NewGuard builds a guard in the given mode (empty defaults to restricted).
func NewGuard(mode SecurityMode, allowed []string) *Guard {
	if mode == "" {
		mode = ModeRestricted
	}
	return &Guard{Mode: mode, AllowedHosts: allowed, resolver: net.DefaultResolver}
}

// CheckURL validates the scheme and host policy of a URL string. IP-level
// checks happen at dial time (see safeDialContext) to defeat DNS rebinding.
func (g *Guard) CheckURL(scheme, host string) error {
	scheme = strings.ToLower(scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("scheme %q not allowed (http/https only)", scheme)
	}
	if g.Mode == ModeAllowlist && !g.hostAllowed(host) {
		return fmt.Errorf("host %q not in allowlist", host)
	}
	return nil
}

func (g *Guard) hostAllowed(host string) bool {
	host = strings.ToLower(hostOnly(host))
	for _, a := range g.AllowedHosts {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" {
			continue
		}
		if host == a || strings.HasSuffix(host, "."+a) {
			return true
		}
	}
	return false
}

// safeDialContext resolves the target host, rejects the connection if any
// resolved address is private/loopback/link-local, and dials the exact
// validated IP so DNS cannot be re-resolved to a blocked address afterwards.
func (g *Guard) safeDialContext(dialTimeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: dialTimeout}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := g.resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", host, err)
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("no addresses for %s", host)
		}
		return dialFirstAllowed(ctx, dialer, network, port, ips)
	}
}

// dialFirstAllowed dials the first resolved address that passes checkIP, so a
// hostname is only ever connected to a validated public IP.
func dialFirstAllowed(ctx context.Context, dialer *net.Dialer, network, port string, ips []net.IPAddr) (net.Conn, error) {
	lastErr := fmt.Errorf("no usable address")
	for _, ip := range ips {
		if err := checkIP(ip.IP); err != nil {
			lastErr = err
			continue
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if err != nil {
			lastErr = err
			continue
		}
		return conn, nil
	}
	return nil, lastErr
}

// checkIP rejects addresses that must never be reachable from the fetcher.
func checkIP(ip net.IP) error {
	if ip == nil {
		return fmt.Errorf("nil ip")
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast() || ip.IsPrivate() {
		return fmt.Errorf("blocked address %s (private/loopback/link-local)", ip)
	}
	// ip.IsPrivate covers RFC1918 and fc00::/7; the checks above cover 127/8,
	// 169.254/16, ::1, fe80::/10, 0.0.0.0. Reject IPv4-mapped private too.
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 127 || (v4[0] == 169 && v4[1] == 254) || v4[0] == 0 {
			return fmt.Errorf("blocked address %s", ip)
		}
	}
	return nil
}

// checkRedirect enforces the redirect hop limit and re-validates the scheme and
// allowlist policy at every hop. IP checks run again at dial time.
func (g *Guard) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	return g.CheckURL(req.URL.Scheme, req.URL.Host)
}

func hostOnly(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

const maxRedirects = 5
