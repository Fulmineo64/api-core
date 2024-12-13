package controller

import (
	"api_core/query"
	"api_core/types/data"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"
	"gorm.io/gorm"
)

type Response struct {
	Data  interface{}
	Next  string
	Count int64
}

func HandleGet(w http.ResponseWriter, r *http.Request, db *gorm.DB, primaries map[string]interface{}, model any) error {
	args := query.QueryMapArgs{
		Sel:       r.URL.Query().Get("sel"),
		Rel:       r.URL.Query().Get("rel"),
		Params:    r.URL.Query().Get("params"),
		P:         r.URL.Query().Get("p"),
		PagStart:  r.URL.Query().Get("pagStart"),
		PagEnd:    r.URL.Query().Get("pagEnd"),
		Ord:       r.URL.Query().Get("ord"),
		Primaries: primaries,
		Model:     model,
	}
	err := query.QueryMap(r, db, &args, query.QueryMapConfig{})
	if err != nil {
		return err
	}
	err = WriteQueryMapResult(w, r, &args)
	if err != nil {
		return err
	}
	return nil
}

func WriteQueryMapResult(w http.ResponseWriter, r *http.Request, args *query.QueryMapArgs) error {
	w.Header().Set("X-Total-Count", strconv.Itoa(int(args.Count)))
	var link string
	if query.ShouldPaginate(args.PagStart, args.PagEnd) {
		limit := query.GetLimit(args.PagStart, args.PagEnd)
		start := query.GetOffset(args.PagStart) + limit
		end := start + limit
		if int64(end) < args.Count {
			var params []string
			for key, values := range r.URL.Query() {
				if key == "pagStart" {
					params = append(params, key+"="+strconv.Itoa(start))
				} else if key == "pagEnd" {
					params = append(params, key+"="+strconv.Itoa(end))
				} else {
					params = append(params, key+"="+strings.Join(values, "&"+key+"="))
				}
			}
			link = r.URL.RequestURI() + "?" + strings.Join(params, "&")
			w.Header().Set("Link", link)
		}
	}
	switch r.Header.Get("Accept") {
	case "application/csv", "text/csv":
		w.Header().Set("Content-Type", r.Header.Get("Accept")+"; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=data.csv")
		render.Status(r, http.StatusOK)

		// TODO: Done but needs better checking; sorting based on the input select
		// TODO: Manage the CSV in the correct order

		var csvData [][]string
		tmz := r.Header.Get("Timezone")
		var heading []string
		for _, f := range args.Info.Fields {
			heading = append(heading, f.Name)
		}
		for key := range args.Info.Nested {
			heading = append(heading, key)
		}
		// for i := range heading {
		// 	if strings.Contains(heading[i], "AS") {
		// 		s := strings.Split(heading[i], "AS")
		// 		heading[i] = strings.TrimSpace(s[len(s)-1])
		// 	}
		// }
		l := len(args.Result)
		if l > 0 {

			csvData = append(csvData, heading)

			for i := 0; i < l; i++ {
				item := args.Result[i]
				var row []string
				for _, f := range args.Info.Fields {
					rv := reflect.ValueOf(item[f.Name])
					if rv.IsValid() && !rv.IsZero() && !rv.IsNil() {
						t := reflect.Indirect(rv)
						if t.Type().Kind() == reflect.Ptr {
							t = t.Elem()
						}
						f := t.Interface()
						if f != nil && f != "" {
							if date, ok := f.(data.Date); ok {
								row = append(row, time.Time(date).Format("02/01/2006"))
							} else if datetime, ok := f.(data.Datetime); ok {
								if r.Header.Get("Only-Date") == "" {
									loc, _ := time.LoadLocation(tmz)
									row = append(row, time.Time(datetime).In(loc).Format("02/01/2006 15:04"))
								} else {
									row = append(row, time.Time(datetime).Format("02/01/2006"))
								}
							} else if _, ok := f.(data.RoundedFloat); ok {
								row = append(row, strings.ReplaceAll(fmt.Sprint(f), ".", ","))
							} else if _, ok := f.(float32); ok {
								row = append(row, strings.ReplaceAll(fmt.Sprint(f), ".", ","))
							} else if _, ok := f.(float64); ok {
								row = append(row, strings.ReplaceAll(fmt.Sprint(f), ".", ","))
							} else {
								if _, ok := f.(string); ok {
									row = append(row, fmt.Sprintf("%s", f))
								} else {
									row = append(row, fmt.Sprint(f))
								}
							}
						} else {
							row = append(row, "")
						}
					} else {
						row = append(row, "")
					}
				}
				for key := range args.Info.Nested {
					data, err := json.Marshal(item[key])
					if err != nil {
						return err
					}
					row = append(row, string(data))
				}
				csvData = append(csvData, row)
			}
		}
		if err := csv.NewWriter(w).WriteAll(csvData); err != nil {
			return err
		}
	case "application/xml", "text/xml":
		if reflect.TypeOf(args.Result).Name() == "" || len(r.URL.Query().Get("wrap")) > 0 {
			render.XML(w, r, Response{Data: args.Result, Next: link, Count: args.Count})
		} else {
			render.XML(w, r, args.Result)
		}
	default:
		var result any = args.Result
		if len(args.Primaries) != 0 {
			result = args.Result[0]
			// TODO: It might be advisable to set Count to 1 in this situation
		}
		if len(r.URL.Query().Get("wrap")) > 0 {
			render.JSON(w, r, Response{Data: result, Next: link, Count: args.Count})
		} else {
			render.JSON(w, r, result)
		}
	}
	return nil
}

func WriteDataWithCount(w http.ResponseWriter, r *http.Request, pagStart, pagEnd string, data any, count int64) error {
	w.Header().Set("X-Total-Count", strconv.Itoa(int(count)))
	var link string
	if query.ShouldPaginate(pagStart, pagEnd) {
		limit := query.GetLimit(pagStart, pagEnd)
		start := query.GetOffset(pagStart) + limit
		end := start + limit
		if int64(end) < count {
			var params []string
			for key, values := range r.URL.Query() {
				if key == "pagStart" {
					params = append(params, key+"="+strconv.Itoa(start))
				} else if key == "pagEnd" {
					params = append(params, key+"="+strconv.Itoa(end))
				} else {
					params = append(params, key+"="+strings.Join(values, "&"+key+"="))
				}
			}
			link = r.URL.RequestURI() + "?" + strings.Join(params, "&")
			w.Header().Set("Link", link)
		}
	}
	switch r.Header.Get("Accept") {
	case "application/csv", "text/csv":
		w.Header().Set("Content-Type", r.Header.Get("Accept")+"; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=data.csv")
		render.Status(r, http.StatusOK)

		var csvData [][]string
		v := reflect.ValueOf(data).Elem()
		t := v.Type()
		if t.Kind() != reflect.Slice {
			t = reflect.SliceOf(t)
			v = reflect.Append(reflect.New(t).Elem(), v)
		}

		l := v.Len()
		if l > 0 {

			t = t.Elem()
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
			}

			var row []string
			for j := 0; j < t.NumField(); j++ {
				row = append(row, t.Field(j).Name)
			}
			csvData = append(csvData, row)

			for i := 0; i < l; i++ {
				item := v.Index(i).Elem()
				ti := item.Type()
				var row []string
				for j := 0; j < ti.NumField(); j++ {
					f := item.Field(j)
					if f.IsValid() && !f.IsZero() {
						if f.Type().Kind() == reflect.Ptr {
							f = f.Elem()
						}
						if f.Type().Kind() == reflect.Slice || f.Type().Kind() == reflect.Array {
							v, _ := json.Marshal(f.Interface())
							row = append(row, string(v))
						} else if marshaler, ok := f.Interface().(json.Marshaler); ok {
							v, _ := marshaler.MarshalJSON()
							row = append(row, strings.TrimSuffix(strings.TrimPrefix(string(v), "\""), "\""))
						} else if stringer, ok := f.Interface().(fmt.Stringer); ok {
							row = append(row, stringer.String())
						} else {
							row = append(row, fmt.Sprintf("%v", f.Interface()))
						}
					} else {
						row = append(row, "")
					}
				}
				csvData = append(csvData, row)
			}
		}
		if err := csv.NewWriter(w).WriteAll(csvData); err != nil {
			return err
		}
	case "application/xml", "text/xml":
		if reflect.TypeOf(data).Name() == "" || len(r.URL.Query().Get("wrap")) > 0 {
			render.XML(w, r, Response{Data: data, Next: link, Count: count})
		} else {
			render.XML(w, r, data)
		}
	default:
		if len(r.URL.Query().Get("wrap")) > 0 {
			render.JSON(w, r, Response{Data: data, Next: link, Count: count})
		} else {
			render.JSON(w, r, data)
		}
	}
	return nil
}
