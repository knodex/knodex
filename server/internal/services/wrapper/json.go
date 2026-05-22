// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package wrapper

import "encoding/json"

// jsonStdMarshal is the standard JSON marshaler. Wrapped behind a package-local
// variable so tests can inject failures without depending on encoding/json directly.
func jsonStdMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
