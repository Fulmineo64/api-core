package permissions

type PermissionsProvider interface {
	PermissionsGet() bool
}
