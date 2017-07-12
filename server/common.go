package server

func removeSubFromList(sub *subscription, sl []*subscription) ([]*subscription, bool) {
	for i := 0; i < len(sl); i++ {
		if sl[i] == sub {
			last := len(sl) - 1
			sl[i] = sl[last]
			sl[last] = nil
			sl = sl[:last]
			return shrinkAsNeeded(sl), true
		}
	}
	return sl, false
}

// Checks if we need to do a resize. This is for very large growth then
// subsequent return to a more normal size from unsubscribe.
func shrinkAsNeeded(sl []*subscription) []*subscription {
	lsl := len(sl)
	csl := cap(sl)
	// Don't bother if list not too big
	if csl <= 8 {
		return sl
	}
	pFree := float32(csl-lsl) / float32(csl)
	if pFree > 0.50 {
		return append([]*subscription(nil), sl...)
	}
	return sl
}
