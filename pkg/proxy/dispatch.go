package proxy

type dispFunc func(s *Slot, r *Request) *SharedBackendConn

func dispReadReplica(s *Slot, r *Request) *SharedBackendConn {
	if r.Dirty || s.migrate != nil {
		return s.backend
	}
	seed := uint(r.Start) % 1024
	for _, group := range s.replica {
		for i := range group {
			k := (int(seed) + i) % len(group)
			if bc := group[k]; bc != nil && bc.IsConnected() {
				return bc
			}
		}
	}
	return s.backend
}
