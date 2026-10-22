// Command postman generates a Postman collection from api/openapi.yaml.
// Usage: go run ./cmd/postman > docs/postman/matritsa.postman_collection.json
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

type kv struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type request struct {
	Method string `json:"method"`
	Header []kv   `json:"header"`
	Body   *body  `json:"body,omitempty"`
	URL    url    `json:"url"`
}

type body struct {
	Mode    string         `json:"mode"`
	Raw     string         `json:"raw,omitempty"`
	Options map[string]any `json:"options,omitempty"`
}

type url struct {
	Raw   string   `json:"raw"`
	Host  []string `json:"host"`
	Path  []string `json:"path"`
	Query []kv     `json:"query,omitempty"`
}

type event struct {
	Listen string `json:"listen"`
	Script struct {
		Type string   `json:"type"`
		Exec []string `json:"exec"`
	} `json:"script"`
}

type item struct {
	Name    string   `json:"name"`
	Request *request `json:"request,omitempty"`
	Item    []item   `json:"item,omitempty"`
	Event   []event  `json:"event,omitempty"`
}

func example(s *openapi3.SchemaRef) any {
	if s == nil || s.Value == nil {
		return nil
	}
	v := s.Value
	if v.Example != nil {
		return v.Example
	}
	switch {
	case v.Type.Is("object"):
		m := map[string]any{}
		names := make([]string, 0, len(v.Properties))
		for k := range v.Properties {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			req := false
			for _, r := range v.Required {
				req = req || r == k
			}
			if req {
				m[k] = example(v.Properties[k])
			}
		}
		return m
	case v.Type.Is("array"):
		return []any{example(v.Items)}
	case v.Type.Is("integer"), v.Type.Is("number"):
		return 0
	case v.Type.Is("boolean"):
		return false
	}
	if len(v.Enum) > 0 {
		return v.Enum[0]
	}
	return "string"
}

func main() {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile("api/openapi.yaml")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	folders := map[string][]item{}
	var order []string
	paths := make([]string, 0, len(doc.Paths.Map()))
	for p := range doc.Paths.Map() {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		pi := doc.Paths.Find(p)
		for _, method := range []string{"GET", "POST", "PATCH", "PUT", "DELETE"} {
			op := pi.GetOperation(method)
			if op == nil {
				continue
			}
			tag := "other"
			if len(op.Tags) > 0 {
				tag = op.Tags[0]
			}
			if _, ok := folders[tag]; !ok {
				order = append(order, tag)
			}
			segs := []string{"api", "v1"}
			for _, s := range strings.Split(strings.Trim(p, "/"), "/") {
				if strings.HasPrefix(s, "{") {
					s = "{{" + strings.Trim(s, "{}") + "}}"
				}
				segs = append(segs, s)
			}
			r := &request{Method: method, URL: url{Raw: "{{baseUrl}}/" + strings.Join(segs, "/"), Host: []string{"{{baseUrl}}"}, Path: segs}}
			public := op.Security != nil && len(*op.Security) == 0
			if !public {
				r.Header = append(r.Header, kv{"Authorization", "Bearer {{token}}"})
			}
			for _, pr := range append(append(openapi3.Parameters{}, pi.Parameters...), op.Parameters...) {
				switch pr.Value.In {
				case "header":
					if pr.Value.Required {
						r.Header = append(r.Header, kv{pr.Value.Name, "{{$guid}}"})
					}
				case "query":
					val := ""
					if pr.Value.Schema != nil && pr.Value.Schema.Value != nil && pr.Value.Schema.Value.Example != nil {
						val = fmt.Sprint(pr.Value.Schema.Value.Example)
					}
					r.URL.Query = append(r.URL.Query, kv{pr.Value.Name, val})
					r.URL.Raw += map[bool]string{true: "?", false: "&"}[len(r.URL.Query) == 1] + pr.Value.Name + "=" + val
				}
			}
			if op.RequestBody != nil {
				for ct, mt := range op.RequestBody.Value.Content {
					if ct == "application/json" {
						r.Header = append(r.Header, kv{"Content-Type", "application/json"})
						b, _ := json.MarshalIndent(example(mt.Schema), "", "  ")
						r.Body = &body{Mode: "raw", Raw: string(b), Options: map[string]any{"raw": map[string]string{"language": "json"}}}
					}
				}
			}
			it := item{Name: op.Summary, Request: r}
			if op.OperationID == "login" {
				var e event
				e.Listen = "test"
				e.Script.Type = "text/javascript"
				e.Script.Exec = []string{"pm.collectionVariables.set('token', pm.response.json().accessToken);"}
				it.Event = []event{e}
				r.Body.Raw = "{\n  \"login\": \"client1\",\n  \"password\": \"demo1234\"\n}"
			}
			folders[tag] = append(folders[tag], it)
		}
	}
	var items []item
	for _, t := range order {
		items = append(items, item{Name: t, Item: folders[t]})
	}
	out := map[string]any{
		"info": map[string]string{
			"name":        "Матрица — кредитный кабинет",
			"description": "Сгенерировано из api/openapi.yaml. Сначала выполните auth/login — токен сохранится в переменной token. Для сотрудника поменяйте логин на staff1.",
			"schema":      "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
		"variable": []kv{{"baseUrl", "http://localhost:3000"}, {"token", ""}, {"id", "<uuid заявки>"}},
		"item":     items,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(out)
}
