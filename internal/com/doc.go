// Package com provides composable HTTP transports that modify outgoing requests.
//
// The package follows a layered transport pattern: each [http.RoundTripper]
// wraps another and adds one concern (base URL resolution, basic auth, or a
// User-Agent header) before delegating to the next layer.
//
// The [http.RoundTripper] contract requires that a transport must not modify
// the caller's request, so each layer clones the request before modifying
// it. In a fully stacked configuration this produces one clone per layer
// rather than one clone total. For the current workload this is considered to
// be acceptable: clones do not copy the request body, earlier clones become
// eligible for garbage collection before the network round-trip begins,
// and request header sets are small.
//
// TODO: If high-concurrency API workloads cause memory pressure, consider
// cloning the request exactly once at the outermost layer and letting inner
// transports mutate the already-cloned copy.
package com
