package interfaces

type Registry interface {
	RegisterModels() []any
	RegisterBasicControllers() []BasicController
	RegisterTypedControllers() []TypedController[any]
}
