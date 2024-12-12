package utils

import (
	"fmt"
	"reflect"
)

func Val[T any](ptr *T) T {
	if ptr == nil {
		return *new(T)
	}
	return *ptr
}

func Ptr[T any](val T) *T {
	return &val
}

func IsZero[T comparable](v T) bool {
	return v == *new(T)
}

// CoalesceAny returns the first non-zero value of the list
func CoalesceAny[T comparable](v T, alt any) any {
	if IsZero(v) {
		return alt
	}
	return v
}

// Coalesce is like CoalesceAny but accepts and returns only values of the specified type, if none of the values supplied are valid it returns a zero value
func Coalesce[T comparable](values ...T) T {
	for i := range values {
		if !IsZero(values[i]) {
			return values[i]
		}
	}
	return *new(T)
}

func If[T any](condition bool, valueThen, valueElse T) T {
	if condition {
		return valueThen
	}
	return valueElse
}

func Substr(source string, from, to int) string {
	if len(source) >= to {
		return source[from:to]
	}
	return source[from:]
}

// Concat merges the values together into a string
func Concat(values ...any) string {
	var str string
	for _, value := range values {
		if reflect.TypeOf(value).Kind() == reflect.Ptr {
			rv := reflect.ValueOf(value)
			if rv.IsNil() {
				value = ""
			} else {
				value = rv.Elem().Interface()
			}
		}
		str += fmt.Sprintf("%v", value)
	}
	return str
}
