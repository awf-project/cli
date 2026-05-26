package agents

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// mcpProxyNamePrefix is the well-known prefix for ephemeral MCP server names
// AWF registers in Gemini/OpenCode CLIs. The purge routine matches on this
// prefix to clean orphans from crashed prior runs.
//
// Using a unique suffix per registration (see randShortID) prevents collisions
// when multiple AWF processes run concurrently: each run registers its own
// namespaced server and removes exactly that server on cleanup, without touching
// registrations owned by other concurrent runs.
const mcpProxyNamePrefix = "awf-proxy-"

// randShortID returns a hex string of length n*2 derived from crypto/rand.
// Used to namespace ephemeral MCP server registrations per step,
// preventing collisions between concurrent runs and orphan reuse.
//
// crypto/rand is used rather than math/rand to avoid PRNG seeding pitfalls and
// to ensure uniqueness even under rapid sequential calls. A failure of
// crypto/rand is catastrophic and extremely rare on modern systems; the
// fallback encodes UnixNano so callers still get a usable (though weaker)
// identifier rather than a zero-length string.
func randShortID(n int) string {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		// crypto/rand failure is catastrophic; fall back to time-nanos hex.
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
