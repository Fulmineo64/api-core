package dialectors

type PostgresDialector struct {
}

func (PostgresDialector) EscapeField(fieldName string) string {
	return `"` + fieldName + `"`
}

func (PostgresDialector) AliasRegex() string {
	return `"\w*"`
}
