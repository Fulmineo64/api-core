package dialectors

type Dialector interface {
	EscapeField(fieldName string) string
}
