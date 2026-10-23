package fake

// Script queues outcomes for the next submissions of kind (DRIVER, VEHICLE,
// TRIP or DAILY_REPORT), consumed first in, first out. A nil outcome means
// accepted; any other error is returned as is (e.g. *regulator.Rejection or
// regulator.ErrUnavailable). Scripted outcomes win over triggers and the mode.
// Safe for concurrent use.
func (f *Regulator) Script(kind string, outcomes ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scripts[kind] = append(f.scripts[kind], outcomes...)
}

// Pending returns how many scripted outcomes are still queued for kind.
func (f *Regulator) Pending(kind string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.scripts[kind])
}

// outcome is one scripted answer; err nil means accepted.
type outcome struct{ err error }

// next pops the next scripted outcome for kind, if any.
func (f *Regulator) next(kind string) (outcome, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	queue := f.scripts[kind]
	if len(queue) == 0 {
		return outcome{}, false
	}
	f.scripts[kind] = queue[1:]
	return outcome{err: queue[0]}, true
}
