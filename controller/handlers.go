package controller

import (
	"api_core/app/dialectors"
	"api_core/datatypes"
	"api_core/query"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Response struct {
	Data  interface{}
	Next  string
	Count int64
}

func HandleGet(c *gin.Context, db *gorm.DB, primaries map[string]interface{}, model any) error {
	if db == nil {
		return errors.New("please provide a valid instance of *gorm.DB in the db param")
	}
	dialector, err := dialectors.ByDB(db)
	if err != nil {
		return err
	}
	args := query.QueryArgs{
		Sel:       c.Query("sel"),
		Rel:       c.Query("rel"),
		Params:    c.Query("params"),
		P:         c.Query("p"),
		PagStart:  c.Query("pagStart"),
		PagEnd:    c.Query("pagEnd"),
		Ord:       c.Query("ord"),
		Primaries: primaries,
		Model:     model,
	}
	err = query.Query(c, db, &args, query.QueryConfig{
		Dialector: dialector,
	})
	if err != nil {
		return err
	}
	err = WriteQueryMapResult(c, &args)
	if err != nil {
		return err
	}
	return nil
}

func WriteQueryMapResult(c *gin.Context, args *query.QueryArgs) error {
	c.Header("X-Total-Count", strconv.Itoa(int(args.Count)))
	var link string
	if query.ShouldPaginate(args.PagStart, args.PagEnd) {
		limit := query.GetLimit(args.PagStart, args.PagEnd)
		start := query.GetOffset(args.PagStart) + limit
		end := start + limit
		if int64(end) < args.Count {
			var params []string
			for key, values := range c.Request.URL.Query() {
				switch key {
				case "pagStart":
					params = append(params, key+"="+strconv.Itoa(start))
				case "pagEnd":
					params = append(params, key+"="+strconv.Itoa(end))
				default:
					params = append(params, key+"="+strings.Join(values, "&"+key+"="))
				}
			}
			link = c.FullPath() + "?" + strings.Join(params, "&")
			c.Header("Link", link)
		}
	}
	switch c.GetHeader("Accept") {
	case "application/csv", "text/csv":
		c.Header("Content-Type", c.GetHeader("Accept")+"; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=data.csv")
		c.Status(http.StatusOK)

		// TODO: Done but needs better checking; sorting based on the input select
		// TODO: Manage the CSV in the correct order

		var csvData [][]string
		tmz := c.GetHeader("Timezone")
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
							if date, ok := f.(datatypes.Date); ok {
								row = append(row, time.Time(date).Format("02/01/2006"))
							} else if datetime, ok := f.(datatypes.Datetime); ok {
								if c.GetHeader("Only-Date") == "" {
									loc, _ := time.LoadLocation(tmz)
									row = append(row, time.Time(datetime).In(loc).Format("02/01/2006 15:04"))
								} else {
									row = append(row, time.Time(datetime).Format("02/01/2006"))
								}
							} else if _, ok := f.(datatypes.RoundedFloat); ok {
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
		if err := csv.NewWriter(c.Writer).WriteAll(csvData); err != nil {
			return err
		}
	case "application/xml", "text/xml":
		if reflect.TypeOf(args.Result).Name() == "" || len(c.Query("wrap")) > 0 {
			c.XML(http.StatusOK, Response{Data: args.Result, Next: link, Count: args.Count})
		} else {
			c.XML(http.StatusOK, args.Result)
		}
	default:
		var result any = args.Result
		if len(args.Primaries) != 0 {
			result = args.Result[0]
			// TODO: It might be advisable to set Count to 1 in this situation
		}
		if len(c.Query("wrap")) > 0 {
			c.JSON(http.StatusOK, Response{Data: result, Next: link, Count: args.Count})
		} else {
			c.JSON(http.StatusOK, result)
		}
	}
	return nil
}

func WriteDataWithCount(c *gin.Context, pagStart, pagEnd string, data any, count int64) error {
	c.Header("X-Total-Count", strconv.Itoa(int(count)))
	var link string
	if query.ShouldPaginate(pagStart, pagEnd) {
		limit := query.GetLimit(pagStart, pagEnd)
		start := query.GetOffset(pagStart) + limit
		end := start + limit
		if int64(end) < count {
			var params []string
			for key, values := range c.Request.URL.Query() {
				switch key {
				case "pagStart":
					params = append(params, key+"="+strconv.Itoa(start))
				case "pagEnd":
					params = append(params, key+"="+strconv.Itoa(end))
				default:
					params = append(params, key+"="+strings.Join(values, "&"+key+"="))
				}
			}
			link = c.FullPath() + "?" + strings.Join(params, "&")
			c.Header("Link", link)
		}
	}
	switch c.GetHeader("Accept") {
	case "application/csv", "text/csv":
		c.Header("Content-Type", c.GetHeader("Accept")+"; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=data.csv")
		c.Status(http.StatusOK)

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
		if err := csv.NewWriter(c.Writer).WriteAll(csvData); err != nil {
			return err
		}
	case "application/xml", "text/xml":
		if reflect.TypeOf(data).Name() == "" || len(c.Query("wrap")) > 0 {
			c.XML(http.StatusOK, Response{Data: data, Next: link, Count: count})
		} else {
			c.XML(http.StatusOK, data)
		}
	default:
		if len(c.Query("wrap")) > 0 {
			c.JSON(http.StatusOK, Response{Data: data, Next: link, Count: count})
		} else {
			c.JSON(http.StatusOK, data)
		}
	}
	return nil
}
