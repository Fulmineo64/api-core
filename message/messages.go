package message

import (
	"net/http"
	"strings"
)

// 2** - Success

// 200
func Ok(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Task performed successfully"),
		Status:  http.StatusOK,
	}
}

// 3** - User error

// 307
func ItemFound(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Item found"),
		Status:  http.StatusTemporaryRedirect,
	}
}

// 4** - User error

// 400
func BadRequest(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Bad request"),
		Status:  http.StatusBadRequest,
	}
}

func InvalidUrlParameter(r *http.Request, parameter string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Missing or invalid required %s field in URL", parameter),
		Status:  http.StatusBadRequest,
	}
}

// 403
func Forbidden(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Forbidden"),
		Status:  http.StatusForbidden,
	}
}

func InsufficientPermissions(r *http.Request, permissions ...string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Permissions %s are required to access this resource, please contact your administrator", strings.Join(permissions, ",")),
		Status:  http.StatusForbidden,
	}
}

func InsufficientPermissionsHasOne(r *http.Request, permissions ...string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Permissions %s are required to access this resource, please contact your administrator", strings.Join(permissions, ",")),
		Status:  http.StatusForbidden,
	}
}

func UnauthorizedRelations(r *http.Request, relations ...string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("You do not have sufficient permissions to access the following relations: %s", strings.Join(relations, ",")),
		Status:  http.StatusForbidden,
	}
}

// 404
func ItemNotFound(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The requested item was not found"),
		Status:  http.StatusNotFound,
	}
}

func FileNotFound(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The requested file was not found"),
		Status:  http.StatusNotFound,
	}
}

func FolderNotFound(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The requested folder was not found"),
		Status:  http.StatusNotFound,
	}
}

// 409
func Conflict(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Conflict"),
		Status:  http.StatusConflict,
	}
}

func DeleteFailed(r *http.Request, blockingRelations []string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Cannot delete the requested resource because it belongs to the following relations.<br>%s", strings.Join(blockingRelations, "<br>")),
		Status:  http.StatusConflict,
	}
}

func DuplicateUnique(r *http.Request, table, combination string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The combination %s already exists for %s", combination, table),
		Status:  http.StatusConflict,
	}
}

func CannotDeleteSharedResource(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Cannot delete the shared resource as you aren't its owner"),
		// You cannot delete the shared resource because you are not the owner
		Status: http.StatusConflict,
	}
}

func ConflictingPaginationAndAggregation(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Pagination is not supported with aggregations"),
		Status:  http.StatusConflict,
	}
}

func ConflictingOrderByAndDistinct(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The Order field must be specified in the Select field when using Distinct"),
		Status:  http.StatusConflict,
	}
}

func ConflictingUsername(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The username is already registered"),
		Status:  http.StatusConflict,
	}
}

func ManualPagination(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("To paginate this request you must specify the Order attribute manually via the 'ord' param"),
		Status:  http.StatusConflict,
	}
}

func MissingRequiredParameter(r *http.Request, name, in string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Missing required parameter %s in %s", name, in),
		Status:  http.StatusConflict,
	}
}

func MissingRequiredParameterForQueryField(r *http.Request, name, field string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Missing required parameter %s in query (eg. &%s=3) for query field %s", name, field),
		Status:  http.StatusConflict,
	}
}

func MissingForeignKey(r *http.Request, key, rel string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Could not find the foreign key %s, required by the relation %s, in its parent object.", key, rel),
		Status:  http.StatusConflict,
	}
}

func CannotCreatePrint(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Could not print the warehouseman order print when the order has not been accepted."),
		Status:  http.StatusConflict,
	}
}

func MissingBaseResourceSelect(r *http.Request, baseResource string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Please select at least one element from the base resource %s before accessing nested resources.", baseResource),
		Status:  http.StatusConflict,
	}
}

// 422
func Unprocessable(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The submitted request present invalid or incomplete data"),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidJSON(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Missing or invalid JSON body"),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidParamsJSON(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The supplied params JSON isn't syntactically valid"),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidParamsSyntax(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The request cannot be completed due to invalid params syntax"),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidField(r *http.Request, field string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The requested field %s could not be found", field),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidFieldValue(r *http.Request, field, rules string, value interface{}) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The specified value %v for the field %s must respect these constraits %s", value, field, rules),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidFieldAlias(r *http.Request, alias, field string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The alias %s specified for the field %s is invalid", alias, field),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidParamOperator(r *http.Request, operator string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The params operator %s is not supported", operator),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidParamType(r *http.Request, field string, correctType string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The supplied field \"%s\" needs to be of type \"%s\"", field, correctType),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidRelation(r *http.Request, table string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Missing or invalid specified relation %s", table),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidRelations(r *http.Request, relations ...string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The request cannot be completed due to invalid specified relations: %s", strings.Join(relations, ",")),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidOrders(r *http.Request, orders ...string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The request cannot be completed due to invalid specified order by: %s", strings.Join(orders, ",")),
		Status:  http.StatusUnprocessableEntity,
	}
}

func DuplicateStructField(r *http.Request, field string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Duplicate struct field %s, use an alias to avoid this error (eg. field AS alias)", field),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidFieldRequired(r *http.Request, name string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The %s property is required", name),
		Status:  http.StatusUnprocessableEntity,
	}
}

func RowError(r *http.Request, row int, message string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("Row %d:%s", row, message),
		Status:  http.StatusUnprocessableEntity,
	}
}

func DisplayNameNotSupported(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("This resource doesn't support DISPLAY_NAME"),
		Status:  http.StatusUnprocessableEntity,
	}
}

// 5** - Server error

func InternalServerError(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("An error has occurred"),
		Status:  http.StatusInternalServerError,
	}
}

func ExpectedSlice(r *http.Request) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The supplied parameter isn't of type *[]models.*"),
		Status:  http.StatusInternalServerError,
	}
}

func UnsupportedParamType(r *http.Request, parameter string) Message {
	return &Msg{
		Message: GetPrinter(r).Sprintf("The supplied parameter %s isn't supported yet. This is an error", parameter),
		Status:  http.StatusInternalServerError,
	}
}

// System messages

type SkipDelete struct{}

func (SkipDelete) Error() string {
	return "___SKIP_DELETE___"
}
