package errs

// Error type is used to create constant errors.
type Error string

func (e Error) Error() string { return string(e) }
