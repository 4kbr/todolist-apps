// Package clock menyediakan abstraksi waktu supaya usecase yang bergantung
// pada waktu sekarang (misal expiry token) bisa ditest tanpa time.Sleep.
package clock

import "time"

// Clock interface untuk abstraksi time.Now()
// memudahkan tsting tanpa time.Sleep
type Clock interface {
	Now() time.Time
}

// System menggunakan time.Now() real
type System struct{}

// Now mengembalikan waktu sekarang yang sebenarnya.
func (System) Now() time.Time {
	return time.Now()
}

// Fixed mengembalikan time yang sama (untuk testing)
type Fixed struct {
	T time.Time
}

// Now mengembalikan waktu tetap yang sudah ditentukan, dipakai di test.
func (f Fixed) Now() time.Time {
	return f.T
}
