package xmlrpc

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSimple(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		p := xml.NewDecoder(r.Body)
		se, _ := nextStart(p) // methodResponse
		if se.Name.Local != "methodCall" {
			http.Error(w, "missing methodCall", http.StatusBadRequest)
			return
		}
		se, _ = nextStart(p) // params
		if se.Name.Local != "methodName" {
			http.Error(w, "missing methodName", http.StatusBadRequest)
			return
		}
		var s string
		if e := p.DecodeElement(&s, &se); e != nil {
			http.Error(w, "wrong function name", http.StatusBadRequest)
			return
		}
		if s != "Add" {
			http.Error(w, fmt.Sprintf("want function name %q but got %q", "Add", s), http.StatusBadRequest)
			return
		}
		se, _ = nextStart(p) // params
		if se.Name.Local != "params" {
			http.Error(w, "missing params", http.StatusBadRequest)
			return
		}
		var args []interface{}
		for {
			se, _ = nextStart(p) // param
			if se.Name.Local == "" {
				break
			}
			if se.Name.Local != "param" {
				http.Error(w, "missing param", http.StatusBadRequest)
				return
			}
			se, _ = nextStart(p) // value
			if se.Name.Local != "value" {
				http.Error(w, "missing value", http.StatusBadRequest)
				return
			}
			_, v, err := next(p)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			args = append(args, v)
		}

		if len(args) != 2 {
			http.Error(w, "bad number of arguments", http.StatusBadRequest)
			return
		}
		switch args[0].(type) {
		case int:
		default:
			http.Error(w, "args[0] should be int", http.StatusBadRequest)
			return
		}
		switch args[1].(type) {
		case int:
		default:
			http.Error(w, "args[1] should be int", http.StatusBadRequest)
			return
		}
		w.Write([]byte(`
		<?xml version="1.0"?>
		<methodResponse>
		<params>
			<param>
				<value><int>` + fmt.Sprint(args[0].(int)+args[1].(int)) + `</int></value>
			</param>
		</params>
		</methodResponse>
		`))
	}))
	defer ts.Close()

	client := NewClient()
	v, err := client.Call(ts.URL+"/api", "Add", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	i, ok := v.(int)
	if !ok {
		t.Fatalf("want int64 but got %T: %v", v, v)
	}
	if i != 3 {
		t.Fatalf("want 3 but got %v", v)
	}
}
