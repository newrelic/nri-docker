// Package biz provides business-value metrics from system raw metrics
package biz

func uint64ToPointer(u uint64) *uint64 {
	return &u
}

func float64ToPointer(f float64) *float64 {
	return &f
}
