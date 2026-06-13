package logging

import "time"

// timeNow is a package-level indirection over time.Now so the wall-clock
// formatter can be overridden in tests if needed.
var timeNow = time.Now
