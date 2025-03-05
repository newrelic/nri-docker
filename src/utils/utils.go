package utils

// ToPointer returns a pointer to the value passed as argument.
func ToPointer[T any](value T) *T {
	return &value
}
