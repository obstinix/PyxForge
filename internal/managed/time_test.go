package managed

import "time"

// testTime is the modification time the test servers report, so Range requests apply.
var testTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
