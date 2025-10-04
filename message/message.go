package message

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type Message interface {
	Text(string) Message
	ToMap() map[string]interface{}
	ToJSON() []byte
	JSON(c *gin.Context)
	Abort(c *gin.Context)
	Write(c *gin.Context)
	Error() string
	IsError() bool
	Is400() bool
	Is500() bool
	Set(key string, val interface{}) Message
	Get(key string) interface{}
	Add(errors ...error) Message
}

type Msg struct {
	Message    string
	Status     int
	Properties map[string]interface{}
}

func (m *Msg) Text(text string) Message {
	m.Message = text
	return m
}

func (m *Msg) ToMap() map[string]interface{} {
	mp := gin.H{"message": m.Message}
	if m.Properties != nil {
		for k, v := range m.Properties {
			mp[k] = v
		}
	}
	return mp
}

func (m *Msg) ToJSON() []byte {
	val, _ := json.Marshal(m.ToMap())
	return val
}

func (m *Msg) JSON(c *gin.Context) {
	c.JSON(m.Status, m.ToMap())
}

func (m *Msg) Abort(c *gin.Context) {
	c.AbortWithStatusJSON(m.Status, m.ToMap())
}

func (m *Msg) Write(c *gin.Context) {
	if m.IsError() {
		m.Abort(c)
	} else {
		m.JSON(c)
	}
}

func (m *Msg) Error() string {
	return m.Message
}

func (m *Msg) IsError() bool {
	return m.Is400() || m.Is500()
}

func (m *Msg) Is400() bool {
	return m.Status >= 400 && m.Status < 500
}

func (m *Msg) Is500() bool {
	return m.Status >= 500
}

func (m *Msg) Set(key string, val interface{}) Message {
	if m.Properties == nil {
		m.Properties = map[string]interface{}{}
	}
	m.Properties[key] = val
	return m
}

func (m *Msg) Get(key string) interface{} {
	if m.Properties == nil {
		return nil
	}
	return m.Properties[key]
}

func (m *Msg) Add(errors ...error) Message {
	for _, e := range errors {
		m.Message += "; " + e.Error()
	}
	return m
}

func GetPrinter(c *gin.Context) *message.Printer {
	if c != nil {
		if ctxI18n, ok := c.Get("i18n"); ok {
			if i18n, ok := ctxI18n.(*message.Printer); ok {
				return i18n
			}
		}
	}
	return message.NewPrinter(language.BritishEnglish)
}

func FromError(status int, err error) Message {
	return &Msg{
		Message: err.Error(),
		Status:  status,
	}
}

func New(status int, text string) Message {
	return &Msg{
		Message: text,
		Status:  status,
	}
}
