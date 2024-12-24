package app

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"api_core/message"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var provider = dbSessionProvider{}
var no404Logger = logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{SlowThreshold: 200 * time.Millisecond, Colorful: true, IgnoreRecordNotFoundError: true, LogLevel: logger.Warn})

type SessionModel struct {
	Key        string `gorm:"primaryKey"`
	ExpiresAt  time.Time
	Properties string `gorm:"type:text"`
}

func (s SessionModel) TableName() string {
	return "sessions"
}

type Session struct {
	properties map[string]interface{}
	expiresAt  time.Time
}

func (s *Session) Get(key string) interface{} {
	return s.properties[key]
}

func (s *Session) Set(key string, value interface{}) {
	s.properties[key] = value
}

func (s *Session) RefreshExpiration() {
	s.expiresAt = time.Now().Add(time.Hour * 12)
}

func (s *Session) SetExpired() {
	s.expiresAt = time.Now()
}

func (s *Session) IsExpired() bool {
	return s.expiresAt.Before(time.Now())
}

func (s *Session) Has(permissions ...string) bool {
	for _, perm := range permissions {
		if s.Get("PERMESSO_"+perm) != true {
			return false
		}
	}
	return true
}

func (s *Session) HasOne(permissions ...string) bool {
	for _, perm := range permissions {
		if s.Get("PERMESSO_"+perm) == true {
			return true
		}
	}
	return false
}

func (s *Session) Check(r *http.Request, permissions ...string) message.Message {
	if !s.Has(permissions...) {
		return message.InsufficientPermissions(r, permissions...)
	}
	return nil
}

func (s *Session) CheckOne(r *http.Request, permissions ...string) message.Message {
	if !s.HasOne(permissions...) {
		return message.InsufficientPermissionsHasOne(r, permissions...)
	}
	return nil
}

// Session providers

type dbSessionProvider struct{}

func (sp *dbSessionProvider) retrieve(key string) *Session {
	session := SessionModel{}
	result := DB.Session(&gorm.Session{Logger: no404Logger}).First(&session, "\"key\" = ?", key)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil
	}
	var properties map[string]interface{}
	json.Unmarshal([]byte(session.Properties), &properties)
	return &Session{properties, session.ExpiresAt}
}

func (sp *dbSessionProvider) store(key string, s *Session) {
	props, _ := json.Marshal(s.properties)
	session := SessionModel{
		Key:        key,
		ExpiresAt:  s.expiresAt,
		Properties: string(props),
	}
	DB.Session(&gorm.Session{Logger: no404Logger}).Save(session)
}

func (sp *dbSessionProvider) delete(key string) {
	DB.Where("\"key\" = ?", key).Delete(&SessionModel{})
}

func (sp *dbSessionProvider) clearExpired() {
	DB.Where("expires_at < ?", time.Now()).Delete(&SessionModel{})
}

// Functions
func GetSession(r *http.Request) *Session {
	return FindSession(strings.ReplaceAll(r.Header.Get("Authorization"), "Bearer ", ""))
}

func FindSession(key string) *Session {
	return provider.retrieve(key)
}

func CreateSession() *Session {
	clearExpired()
	s := &Session{properties: make(map[string]interface{})}
	s.RefreshExpiration()
	return s
}

func PutSession(key string, session *Session) {
	provider.store(key, session)
}

func DeleteSession(key string) {
	provider.delete(key)
}

func clearExpired() {
	provider.clearExpired()
}
