package dialectors

type PostgresDialector struct {
}

func (PostgresDialector) EscapeField(fieldName string) string {
	return `"` + fieldName + `"`
}

func (PostgresDialector) ExposeSQLErr(err error) error {
	return nil
}
