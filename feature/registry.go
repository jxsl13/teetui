package feature

import (
	"sync"

	"github.com/jxsl13/twclient/client"
)

type entry struct {
	f        Feature
	disabled bool
	closed   bool // Close already run (idempotent teardown, §V62)
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
			closeFeature(e, host) // release resources of the disabled feature (§V62)
			res = false
		}
	}()
	return fn()
}

// closeFeature runs a feature's optional Close once (idempotent, §V62), recovering
// any panic so teardown of one feature never affects another or the shutdown path.
func closeFeature(e *entry, host Host) (err error) {
	mu.Lock()
	if e.closed {
		mu.Unlock()
		return nil
	}
	e.closed = true
	mu.Unlock()
	c, ok := e.f.(Closer)
	if !ok {
		return nil
	}
	defer func() {
		if rec := recover(); rec != nil && host != nil {
			host.Log("feature " + e.f.Name() + " panicked in Close")
		}
	}()
	return c.Close()
}

// CloseAll runs Close on every registered feature that implements Closer — on
// shutdown — INCLUDING disabled ones, since a feature disabled after a partial
// Init may still hold resources (§V62). Idempotent per feature.
func CloseAll(host Host) []error {
	mu.Lock()
	all := make([]*entry, len(features))
	copy(all, features)
	mu.Unlock()
	var errs []error
	for _, e := range all {
		if err := closeFeature(e, host); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// InitAll runs the optional Init hook of every registered feature once, in
// order. A feature with no Initializer is skipped; one whose Init panics or
// errors is disabled (and its error returned) but the others still init (§V47).
func InitAll(host Host) []error {
	var errs []error
	for _, e := range snapshot() {
		init, hasInit := e.f.(Initializer)
		validator, hasVal := e.f.(Validator)
		if !hasInit && !hasVal {
			continue
		}
		safeCall(e, host, "Init", func() bool {
			if hasInit {
				if err := init.Init(host); err != nil {
					mu.Lock()
					e.disabled = true
					mu.Unlock()
					errs = append(errs, err)
					return false
				}
			}
			// Validate runs after a successful Init; failure disables the feature
			// (§V62) but the others keep running.
			if hasVal {
				if err := validator.Validate(); err != nil {
					mu.Lock()
					e.disabled = true
					mu.Unlock()
					if host != nil {
						host.Log("feature " + e.f.Name() + " failed validation: " + err.Error())
					}
					errs = append(errs, err)
				}
			}
			return false
		})
	}
	return errs
}

// The Fire* helpers dispatch one event to every enabled feature that implements
// the matching handler interface (type-asserted; others skipped, §V60), with
// panic isolation. OnChat suppress is OR across features; OnKey handled stops at
// the first consumer (§V39/§V47).

func FireConnect(h Host) {
	for _, e := range snapshot() {
		if hh, ok := e.f.(ConnectHandler); ok {
			safeCall(e, h, "OnConnect", func() bool { hh.OnConnect(h); return false })
		}
	}
}

func FireDisconnect(h Host, reason string) {
	for _, e := range snapshot() {
		if hh, ok := e.f.(DisconnectHandler); ok {
			safeCall(e, h, "OnDisconnect", func() bool { hh.OnDisconnect(h, reason); return false })
		}
	}
}

func FireChat(h Host, ev ChatEvent) (suppress bool) {
	for _, e := range snapshot() {
		hh, ok := e.f.(ChatHandler)
		if !ok {
			continue
		}
		if safeCall(e, h, "OnChat", func() bool { return hh.OnChat(h, ev) }) {
			suppress = true
		}
	}
	return suppress
}

func FireBroadcast(h Host, text string) {
	for _, e := range snapshot() {
		if hh, ok := e.f.(BroadcastHandler); ok {
			safeCall(e, h, "OnBroadcast", func() bool { hh.OnBroadcast(h, text); return false })
		}
	}
}

func FireServerMsg(h Host, text string) {
	for _, e := range snapshot() {
		if hh, ok := e.f.(ServerMsgHandler); ok {
			safeCall(e, h, "OnServerMsg", func() bool { hh.OnServerMsg(h, text); return false })
		}
	}
}

func FireKill(h Host, ev KillEvent) {
	for _, e := range snapshot() {
		if hh, ok := e.f.(KillHandler); ok {
			safeCall(e, h, "OnKill", func() bool { hh.OnKill(h, ev); return false })
		}
	}
}

func FireTick(h Host, st client.TickState) {
	for _, e := range snapshot() {
		if hh, ok := e.f.(TickHandler); ok {
			safeCall(e, h, "OnTick", func() bool { hh.OnTick(h, st); return false })
		}
	}
}

func FireKey(h Host, k Key) (handled bool) {
	for _, e := range snapshot() {
		hh, ok := e.f.(KeyHandler)
		if !ok {
			continue
		}
		if safeCall(e, h, "OnKey", func() bool { return hh.OnKey(h, k) }) {
			return true
		}
	}
	return false
}
