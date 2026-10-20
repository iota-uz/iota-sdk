package resource

import "fmt"

// ReplaceMany atomically retires same-budget reservations and admits their
// replacement. Failure leaves every reservation and the shared usage unchanged.
func (b *Budget) ReplaceMany(previous []*Reservation, n int64) (*Reservation, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	seen := make(map[*Reservation]struct{}, len(previous))
	var credit int64
	for _, reservation := range previous {
		if reservation == nil {
			continue
		}
		if reservation.budget != b || reservation.released {
			return nil, fmt.Errorf("replacement requires active reservations from the same budget")
		}
		if _, duplicate := seen[reservation]; duplicate {
			continue
		}
		seen[reservation] = struct{}{}
		credit += reservation.bytes
	}
	if n < 0 || n > b.limit-b.used+credit {
		return nil, NewAdmissionError("retained bytes", n, b.limit)
	}
	for reservation := range seen {
		reservation.released = true
	}
	b.used = b.used - credit + n
	return &Reservation{budget: b, bytes: n}, nil
}
