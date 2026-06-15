package feature

import (
	"sync"

	"github.com/jxsl13/twclient/client"
)

type entry struct {
	f        Feature
	disabled bool
}

var (
	mu       sync.Mutex
	features []*entry
)

// Register adds a feature to the global registry. Call from a feature package's
// init(); the feature is activated by main blank-importing that package.
func Register(f Feature) {
	if f == nil {
		return
	}
	mu.Lock()
	features = append(features, &entry{f: f})
	mu.Unlock()
}

// Registered returns the registered (non-disabled) features in registration
// order.
func Registered() []Feature {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Feature, 0, len(features))
	for _, e := range features {
		if !e.disabled {
			out = append(out, e.f)
		}
	}
	return out
}

// Reset clears the registry (test helper).
func Reset() {
	mu.Lock()
	features = nil
	mu.Unlock()
}

// Count returns the number of registered (including disabled) features.
func Count() int {
	mu.Lock()
	defer mu.Unlock()
	return len(features)
}

// snapshot returns the enabled entries (copy, so dispatch holds no lock while
// calling feature code).
func snapshot() []*entry {
	mu.Lock()
	defer mu.Unlock()
	out := make([]*entry, 0, len(features))
	for _, e := range features {
		if !e.disabled {
			out = append(out, e)
		}
	}
	return out
}

// safeCall runs fn for one feature, recovering panics: the feature is disabled
// for the session and logged via host, never crashing the client (§V40/§V47).
func safeCall(e *entry, host Host, what string, fn func() bool) (res bool) {
	defer func() {
		if rec := recover(); rec != nil {
			mu.Lock()
			e.disabled = true
			mu.Unlock()
			if host != nil {
				host.Log("feature " + e.f.Name() + " panicked in " + what + " and was disabled")
			}
			res = false
		}
	}()
	return fn()
}

// ProvisionAll provisions every registered feature once, in order. A feature
// whose Provision panics or errors is disabled (and its error returned) but the
// others still provision (§V47).
func ProvisionAll(host Host) []error {
	var errs []error
	for _, e := range snapshot() {
		safeCall(e, host, "Provision", func() bool {
			if err := e.f.Provision(host); err != nil {
				mu.Lock()
				e.disabled = true
				mu.Unlock()
				errs = append(errs, err)
			}
			return false
		})
	}
	return errs
}

// The Fire* helpers dispatch one event to every enabled feature with panic
// isolation. OnChat suppress is OR across features; OnKey handled stops at the
// first consumer (§V39/§V47).

func FireConnect(h Host) {
	for _, e := range snapshot() {
		safeCall(e, h, "OnConnect", func() bool { e.f.OnConnect(h); return false })
	}
}

func FireDisconnect(h Host, reason string) {
	for _, e := range snapshot() {
		safeCall(e, h, "OnDisconnect", func() bool { e.f.OnDisconnect(h, reason); return false })
	}
}

func FireChat(h Host, ev ChatEvent) (suppress bool) {
	for _, e := range snapshot() {
		if safeCall(e, h, "OnChat", func() bool { return e.f.OnChat(h, ev) }) {
			suppress = true
		}
	}
	return suppress
}

func FireBroadcast(h Host, text string) {
	for _, e := range snapshot() {
		safeCall(e, h, "OnBroadcast", func() bool { e.f.OnBroadcast(h, text); return false })
	}
}

func FireServerMsg(h Host, text string) {
	for _, e := range snapshot() {
		safeCall(e, h, "OnServerMsg", func() bool { e.f.OnServerMsg(h, text); return false })
	}
}

func FireKill(h Host, ev KillEvent) {
	for _, e := range snapshot() {
		safeCall(e, h, "OnKill", func() bool { e.f.OnKill(h, ev); return false })
	}
}

func FireTick(h Host, st client.TickState) {
	for _, e := range snapshot() {
		safeCall(e, h, "OnTick", func() bool { e.f.OnTick(h, st); return false })
	}
}

func FireKey(h Host, k Key) (handled bool) {
	for _, e := range snapshot() {
		if safeCall(e, h, "OnKey", func() bool { return e.f.OnKey(h, k) }) {
			return true
		}
	}
	return false
}
