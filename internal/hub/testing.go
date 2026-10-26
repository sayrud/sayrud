package hub

import "crypto/ed25519"

// UseForTest points the client to a test server and trusts the given keys instead of the pinned ones until restore is
// called. The statements must still be issued by URL. It is meant for the tests of the packages using the Hub.
func UseForTest(serverURL string, trusted ...ed25519.PublicKey) (restore func()) {
	savedURL, savedKeys := baseURL, keys
	baseURL, keys = serverURL, NewKeyRing(trusted...)
	resetUpdatesCache()
	return func() {
		baseURL, keys = savedURL, savedKeys
		resetUpdatesCache()
	}
}

func resetUpdatesCache() {
	updatesCache.Lock()
	defer updatesCache.Unlock()
	updatesCache.key, updatesCache.result = "", nil
}
