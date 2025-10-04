package message

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 2** - Success

// 200
func Ok(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Operazione eseguita con successo"),
		Status:  http.StatusOK,
	}
}

// 3** - User error

// 307
func ItemFound(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Elemento trovato"),
		Status:  http.StatusTemporaryRedirect,
	}
}

// 4** - User error

// 400
func BadRequest(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Richiesta non valida"),
		Status:  http.StatusBadRequest,
	}
}

func InvalidUrlParameter(c *gin.Context, parameter string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Il parametro in URL %s risulta mancante o non valido", parameter),
		Status:  http.StatusBadRequest,
	}
}

// 403
func Forbidden(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Accesso vietato"),
		Status:  http.StatusForbidden,
	}
}

func InsufficientPermissions(c *gin.Context, permissions ...string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("I permessi %s sono richiesti per accedere a questa risorsa, per supporto contatta il tuo amministratore", strings.Join(permissions, ",")),
		Status:  http.StatusForbidden,
	}
}

func InsufficientPermissionsHasOne(c *gin.Context, permissions ...string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Almeno uno di questi permessi %s è richiesto per accedere a questa risorsa, per supporto contatta il tuo amministratore", strings.Join(permissions, ",")),
		Status:  http.StatusForbidden,
	}
}

func UnauthorizedRelations(c *gin.Context, relations ...string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Non hai autorizzazioni sufficienti per accedere alle seguenti relazioni: %s", strings.Join(relations, ",")),
		Status:  http.StatusForbidden,
	}
}

func UnathorizedModel(c *gin.Context, model string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Non puoi accedere al modello: %s", model),
		Status:  http.StatusForbidden,
	}
}

// 404
func ItemNotFound(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("La risorsa richiesta non è stata trovata"),
		Status:  http.StatusNotFound,
	}
}

func FileNotFound(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Il file richiesto non è stato trovato"),
		Status:  http.StatusNotFound,
	}
}

func FolderNotFound(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("La cartella richiesta non è stata trovata"),
		Status:  http.StatusNotFound,
	}
}

// 409
func Conflict(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Conflict"),
		Status:  http.StatusConflict,
	}
}

func DeleteFailed(c *gin.Context, blockingRelations []string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Non è possibile eliminare la risorsa desiderata perché è utilizzata nelle seguenti relazioni.<br>%s", strings.Join(blockingRelations, "<br>")),
		Status:  http.StatusConflict,
	}
}

func DuplicateUnique(c *gin.Context, table, combination string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("La combinazione %s esiste già per %s", combination, table),
		Status:  http.StatusConflict,
	}
}

func CannotDeleteSharedResource(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Impossibile eliminare la risorsa condivisa poiché non ne sei il proprietario"),
		// You cannot delete the shared resource because you are not the owner
		Status: http.StatusConflict,
	}
}

func ConflictingPaginationAndAggregation(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("La paginazione non è supportata con le aggregazioni"),
		Status:  http.StatusConflict,
	}
}

func ConflictingOrderByAndDistinct(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Il campo Ordine deve essere specificato nel campo Select quando si usa Distinct"),
		Status:  http.StatusConflict,
	}
}

func ConflictingUsername(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Username già registrato"),
		Status:  http.StatusConflict,
	}
}

func ManualPagination(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Per paginare la richiesta bisogna specificare l'attributo Order manualmente tramite il parametro in url 'ord'"),
		Status:  http.StatusConflict,
	}
}

func MissingRequiredParameter(c *gin.Context, name, in string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Parametro richiesto %s mancante in %s", name, in),
		Status:  http.StatusConflict,
	}
}

func MissingRequiredParameterForQueryField(c *gin.Context, name, field string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Parametro richiesto %s mancante nella query (es. &%s=3) per campo %s", name, field),
		Status:  http.StatusConflict,
	}
}

func MissingForeignKey(c *gin.Context, key, rel string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Impossibile trovare la chiave esterna %s, richiesta dalla relazione %s, nell'oggetto padre.", key, rel),
		Status:  http.StatusConflict,
	}
}

func CannotCreatePrint(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Impossibile stampare l'ordine del magazziniere quando l'ordine non è stato accettato."),
		Status:  http.StatusConflict,
	}
}

func MissingBaseResourceSelect(c *gin.Context, baseResource string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Seleziona almeno un elemento dalla risorsa base %s prima di accedere alle releazioni nidificate.", baseResource),
		Status:  http.StatusConflict,
	}
}

// 422
func Unprocessable(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("La richiesta inviata contiene dati non validi o incompleti"),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidJSON(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("JSON mancante o non valido"),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidParamsJSON(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Il JSON dei parametri specificato non è sintatticamente corretto"),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidParamsSyntax(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("La richiesta non è potuta essere completata per via di una sintassi dei parametri errata"),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidField(c *gin.Context, field string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Il campo %s da voi richiesto non è stato trovato", field),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidFieldValue(c *gin.Context, field, rules string, value interface{}) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Il valore %v specificato per il campo %s deve rispettare queste condizioni %s", value, field, rules),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidFieldAlias(c *gin.Context, alias, field string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("L'alias %s, specificato per il campo %s, non è valido", alias, field),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidParamOperator(c *gin.Context, operator string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("L'operatore di parametro %s non è supportato", operator),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidParamType(c *gin.Context, field string, correctType string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Il campo \"%s\" deve essere del tipo \"%s\"", field, correctType),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidRelation(c *gin.Context, table string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("La relazione %s non è stata trovata", table),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidRelations(c *gin.Context, relations ...string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Non è stato possibile completare la richiesta per via delle seguenti relazioni non valide specificate: %s", strings.Join(relations, ",")),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidOrders(c *gin.Context, orders ...string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Non è stato possibile completare la richiesta per via dei seguenti ordinamenti non validi specificati: %s", strings.Join(orders, ",")),
		Status:  http.StatusUnprocessableEntity,
	}
}

func DuplicateStructField(c *gin.Context, field string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Campo %s duplicato nella struct, usare un alias per evitare questo errore (es. campo AS alias)", field),
		Status:  http.StatusUnprocessableEntity,
	}
}

func InvalidFieldRequired(c *gin.Context, name string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("La proprietà %s è obbligatoria", name),
		Status:  http.StatusUnprocessableEntity,
	}
}

func RowError(c *gin.Context, row int, message string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Riga %d:%s", row, message),
		Status:  http.StatusUnprocessableEntity,
	}
}

func DisplayNameNotSupported(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Questa risorsa non supporta DISPLAY_NAME"),
		Status:  http.StatusUnprocessableEntity,
	}
}

// 5** - Server error

func InternalServerError(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Si è verificato un errore"),
		Status:  http.StatusInternalServerError,
	}
}

func ExpectedSlice(c *gin.Context) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Il parametro specificato non è del tipo *[]models.*"),
		Status:  http.StatusInternalServerError,
	}
}

func UnsupportedParamType(c *gin.Context, parameter string) Message {
	return &Msg{
		Message: GetPrinter(c).Sprintf("Il parametro specificato %s non è ancora supportato.", parameter),
		Status:  http.StatusInternalServerError,
	}
}

// System messages

type SkipDelete struct{}

func (SkipDelete) Error() string {
	return "___SKIP_DELETE___"
}
