package dialectors

type SqlserverDialector struct {
}

func (SqlserverDialector) EscapeField(fieldName string) string {
	return "[" + fieldName + "]"
}

func (SqlserverDialector) AliasRegex() string {
	return `[\w*]`
}
