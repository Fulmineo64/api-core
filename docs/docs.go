package docs

import (
	"embed"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"
	"unicode"

	"api_core/registry"
	"api_core/utils"

	"github.com/go-chi/render"
)

//go:embed public/*
var DocsFs embed.FS

type OpenAPIV3 struct {
	Openapi      string                               `json:"openapi"`
	Info         OpenAPIV3Info                        `json:"info"`
	Servers      []OpenAPIV3Server                    `json:"servers"`
	Paths        map[string]map[string]OpenAPIV3Route `json:"paths"`
	Components   map[string]interface{}               `json:"components"`
	Security     []map[string][]string                `json:"security"`
	ExternalDocs OpenAPIV3ExternalDoc                 `json:"externalDocs"`
}

type OpenAPIV3Info struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type OpenAPIV3ExternalDoc struct {
	Description string `json:"description"`
	Url         string `json:"url"`
}

type OpenAPIV3Server struct {
	Url         string `json:"url"`
	Description string `json:"description"`
}

type OpenAPIV3Route struct {
	Tags        []string               `json:"tags"`
	Parameters  []interface{}          `json:"parameters"`
	RequestBody *OpenAPIV3RequestBody  `json:"requestBody,omitempty"`
	Responses   map[string]interface{} `json:"responses"`
}

type OpenAPIV3RequestBody struct {
	Description string                        `json:"description"`
	Content     map[string]OpenAPIV3MediaType `json:"content"`
	Required    bool                          `json:"required"`
}

type OpenAPIV3MediaType struct {
	Schema OpenAPIV3Ref `json:"schema"`
}

type OpenAPIV3Ref struct {
	Ref string `json:"$ref"`
}

type OpenAPIV3Parameter struct {
	In          string      `json:"in"`
	Name        string      `json:"name"`
	Schema      interface{} `json:"schema"`
	Required    bool        `json:"required"`
	Description string      `json:"description"`
}

type OpenAPIV3ParameterSchema struct {
	Type string `json:"type"`
}

type OpenAPIV3ParameterSchemaDefault struct {
	Type    string      `json:"type"`
	Default interface{} `json:"default"`
}

type OpenAPIV3Response struct {
	Description string                              `json:"description"`
	Content     map[string]OpenAPIV3ResponseContent `json:"content"`
}

type OpenAPIV3ResponseContent struct {
	Schema interface{} `json:"schema"`
}

type OpenAPIV3SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme"`
	BearerFormat string `json:"bearerFormat"`
}

type DocsOptions struct {
	Name        string
	Description string
	Version     string
	Servers     []OpenAPIV3Server
}

func GetDocs(r *http.Request, options DocsOptions) OpenAPIV3 {
	docs := OpenAPIV3{
		Openapi: "3.1.0",
		Info: OpenAPIV3Info{
			options.Name,
			options.Description,
			utils.Coalesce(options.Version, "1.0.0"),
		},
		Servers: append(
			[]OpenAPIV3Server{{"../", "Current server"}},
			options.Servers...,
		),
		Paths: map[string]map[string]OpenAPIV3Route{},
		Components: map[string]interface{}{
			"securitySchemes": map[string]OpenAPIV3SecurityScheme{
				"bearerAuth": {
					Type:         "http",
					Scheme:       "bearer",
					BearerFormat: "JWT",
				},
			},
			"schemas": map[string]interface{}{
				"Empty": map[string]interface{}{
					"required":   []interface{}{},
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
			"parameters": map[string]OpenAPIV3Parameter{
				"query_pagStart": {
					In:   "query",
					Name: "pagStart",
					Schema: OpenAPIV3ParameterSchemaDefault{
						Type:    "integer",
						Default: 0,
					},
					Description: "Used for pagination, specifies the starting row number.",
				},
				"query_pagEnd": {
					In:   "query",
					Name: "pagEnd",
					Schema: OpenAPIV3ParameterSchemaDefault{
						Type:    "integer",
						Default: 100,
					},
					Description: "Used for pagination, specifies the ending row number.",
				},
				"query_ord": {
					In:   "query",
					Name: "ord",
					Schema: OpenAPIV3ParameterSchema{
						Type: "string",
					},
					Description: "Used to order data, has a syntax like SQL's ORDER BY. [Docs](https://faq.bp2.it/documentazione_api.asp#ord)\n\n*Example*: `CODE DESC,RELATION.NAME`",
				},
				"query_sel": {
					In:   "query",
					Name: "sel",
					Schema: OpenAPIV3ParameterSchema{
						Type: "string",
					},
					Description: "Used to select specific fields of the main resource, relations and nested relations. [Docs](https://faq.bp2.it/documentazione_api.asp#sel)\n\n*Example*: `ID,CODE,RELATION.NAME`",
				},
				"query_rel": {
					In:   "query",
					Name: "rel",
					Schema: OpenAPIV3ParameterSchema{
						Type: "string",
					},
					Description: "Used to select all fields of relations and nested relations. [Docs](https://faq.bp2.it/documentazione_api.asp#rel)\n\n*Example*: `RELATION,OTHER_RELATION.NESTED_RELATION`",
				},
				"query_params": {
					In:   "query",
					Name: "p",
					Schema: OpenAPIV3ParameterSchema{
						Type: "string",
					},
					Description: "Used to filter data through a custom syntax. [Docs](https://faq.bp2.it/documentazione_api.asp#p)\n\n*Example*: `{ \"CODE\": \"005\" }`",
				},
				"query_structureRel": {
					In:   "query",
					Name: "rel",
					Schema: OpenAPIV3ParameterSchema{
						Type: "string",
					},
					Description: "Used to specify the relations and nested relations to load the structure.\n\n*Example*: `RELATION,OTHER_RELATION.NESTED_RELATION`",
				},
				"path_structureRel": {
					In:   "path",
					Name: "rel",
					Schema: OpenAPIV3ParameterSchema{
						Type: "string",
					},
					Required:    true,
					Description: "Used to specify a single relation of the endpoint to load the structure.\n\n*Example*: `RELATION`",
				},
			},
			"responses": map[string]interface{}{
				"200": OpenAPIV3Response{
					Description: "Success",
					Content: map[string]OpenAPIV3ResponseContent{
						"application/json": {
							Schema: OpenAPIV3ParameterSchema{
								Type: "string",
							},
						},
					},
				},
				"401": OpenAPIV3Response{
					Description: "Unauthorized, the token has expired. To solve this error obtain a fresh token and validate it through /v1/auth/login",
					Content: map[string]OpenAPIV3ResponseContent{
						"application/json": {
							Schema: OpenAPIV3ParameterSchema{
								Type: "string",
							},
						},
					},
				},
				"403": OpenAPIV3Response{
					Description: "Forbidden, you do not have the correct permissions to access the desired resource.",
					Content: map[string]OpenAPIV3ResponseContent{
						"application/json": {
							Schema: OpenAPIV3ParameterSchema{
								Type: "string",
							},
						},
					},
				},
				"500": OpenAPIV3Response{
					Description: "Internal server error. Something went while executing the request",
					Content: map[string]OpenAPIV3ResponseContent{
						"application/json": {
							Schema: OpenAPIV3ParameterSchema{
								Type: "string",
							},
						},
					},
				},
			},
		},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
		ExternalDocs: OpenAPIV3ExternalDoc{
			Description: "Learn more about the API",
			Url:         "http://faq.bp2.it/documentazione_api.asp",
		},
	}

	// TODO: Restore hasSessions when all routes are authenticated
	/*hasSession := request.Session(r) != nil*/
	pathParamsReg := regexp.MustCompile(`{(\w+)}`)

	for _, controller := range registry.ControllerByName {
		for _, route := range controller.Routes() {
			if /*(!hasSession ||*/ route.Authenticate(r) != nil /*)*/ {
				continue
			}

			routePath := path.Clean(controller.FullPath(controller) + "/" + route.Pattern)
			paramsResults := pathParamsReg.FindAllStringSubmatch(routePath, -1)
			routePath = pathParamsReg.ReplaceAllString(routePath, "{$1}")

			var currentRoute map[string]OpenAPIV3Route
			if docs.Paths[routePath] == nil {
				docs.Paths[routePath] = map[string]OpenAPIV3Route{}
			}
			currentRoute = docs.Paths[routePath]
			mainTag := endpointToTag(controller.FullPath(controller))
			newRoute := OpenAPIV3Route{
				Tags:       []string{mainTag},
				Parameters: []interface{}{},
				Responses: map[string]interface{}{
					"200": OpenAPIV3Ref{
						Ref: "#/components/responses/200",
					},
					"401": OpenAPIV3Ref{
						Ref: "#/components/responses/401",
					},
					"403": OpenAPIV3Ref{
						Ref: "#/components/responses/403",
					},
					"500": OpenAPIV3Ref{
						Ref: "#/components/responses/500",
					},
				},
			}

			if len(paramsResults) >= 0 && !strings.Contains(routePath, "structure") {
				for _, res := range paramsResults {
					param := OpenAPIV3Parameter{
						In:   "path",
						Name: res[1],
						Schema: OpenAPIV3ParameterSchema{
							Type: "integer",
						},
						Required: true,
					}
					if res[1] == "id" {
						param.Description = "The value of the primary key of this resource"
					}
					newRoute.Parameters = append(newRoute.Parameters, param)
				}
			}

			if route.Method == http.MethodGet {
				if strings.HasSuffix(routePath, "structure") {
					newRoute.Parameters = append(newRoute.Parameters, OpenAPIV3Ref{"#/components/parameters/query_structureRel"})
				} else if strings.HasSuffix(routePath, "structure/{rel}") {
					newRoute.Parameters = append(newRoute.Parameters, OpenAPIV3Ref{"#/components/parameters/path_structureRel"})
				} else {
					// Parameters
					if len(paramsResults) == 0 {
						newRoute.Parameters = append(newRoute.Parameters, OpenAPIV3Ref{"#/components/parameters/query_pagStart"}, OpenAPIV3Ref{"#/components/parameters/query_pagEnd"}, OpenAPIV3Ref{"#/components/parameters/query_ord"})
					}
					newRoute.Parameters = append(newRoute.Parameters, OpenAPIV3Ref{"#/components/parameters/query_sel"}, OpenAPIV3Ref{"#/components/parameters/query_rel"}, OpenAPIV3Ref{"#/components/parameters/query_params"})
				}
			} else if (route.Method == http.MethodPost || route.Method == http.MethodPatch) && routePath != "/v1/auth/login" {
				newRoute.RequestBody = &OpenAPIV3RequestBody{Description: "The JSON of the resource", Content: map[string]OpenAPIV3MediaType{"application/json": {
					Schema: OpenAPIV3Ref{"#/components/schemas/Empty"},
				}}, Required: true}
			}

			currentRoute[strings.ToLower(route.Method)] = newRoute
		}
	}

	return docs
}

func DocsHandler(options DocsOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url := "public" + r.RequestURI
		var contentType string

		if strings.HasSuffix(url, ".html") {
			contentType = "text/html; charset=utf-8"
		} else if strings.HasSuffix(url, ".css") {
			contentType = "text/css; charset=utf-8"
		} else if strings.HasSuffix(url, ".js") {
			contentType = "application/javascript"
		} else if strings.HasSuffix(url, ".json") {
			contentType = "application/json"
		}

		if strings.HasSuffix(url, "/docs.json") {
			render.JSON(w, r, GetDocs(r, options))
		} else {
			f, err := DocsFs.Open(url)
			if err != nil {
				// messages.FileNotFound(c).Abort(c)
				return
			}

			defer f.Close()

			data, err := io.ReadAll(f)
			if err != nil {
				// models.MailError(c, err)
				return
			}

			w.Header().Set("Content-Type", contentType)
			w.WriteHeader(http.StatusOK)
			w.Write(data)
		}
	}
}

func endpointToTag(apiName string) string {
	apiName = strings.TrimPrefix(apiName, "/")
	apiName = strings.ReplaceAll(apiName, "/", " -")
	apiName = strings.Replace(apiName, "-", "", 1)
	var parts []string
	start := 0
	for end, r := range apiName {
		if end != 0 && unicode.IsUpper(r) {
			parts = append(parts, apiName[start:end])
			start = end
		}
	}
	if start != len(apiName) {
		parts = append(parts, apiName[start:])
	}
	return strings.Join(parts, " ")
}
