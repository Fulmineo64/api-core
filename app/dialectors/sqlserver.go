package dialectors

import (
	"api_core/message"
	"net/http"
)

type SqlserverError interface {
	Error() string
	SQLErrorClass() uint8
	SQLErrorLineNo() int32
	SQLErrorMessage() string
	SQLErrorNumber() int32
	SQLErrorProcName() string
	SQLErrorServerName() string
	SQLErrorState() uint8
}

type SqlserverDialector struct {
}

func (SqlserverDialector) EscapeField(fieldName string) string {
	return "[" + fieldName + "]"
}

func (SqlserverDialector) ExposeSQLErr(err error) error {
	if err != nil {
		if mssqlerr, ok := err.(SqlserverError); ok {
			switch mssqlerr.SQLErrorNumber() {

			case 242, /* Invalid nvarchar conversion range */
				245,  /* Cast failed */
				2601, /* Unique constraint violation */
				2627, /* Primary key violation */
				8114 /* Errore durante la conversione del tipo di dati da nvarchar a float. */ :
				return message.FromError(http.StatusConflict, err)
			}
		} /* else if w := errors.Unwrap(err); w != nil {
			if mssqlerr, ok := w.(MSSqlError); ok {
				fmt.Println(mssqlerr.SQLErrorNumber())
			}
		}*/
		return err
	}
	return nil
}
