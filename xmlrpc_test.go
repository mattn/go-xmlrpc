package xmlrpc

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func createServer(path, name string, f func(args ...interface{}) (interface{}, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
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
		if err := p.DecodeElement(&s, &se); err != nil {
			http.Error(w, "wrong function name", http.StatusBadRequest)
			return
		}
		if s != name {
			http.Error(w, fmt.Sprintf("want function name %q but got %q", name, s), http.StatusBadRequest)
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

		ret, err := f(args...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Write([]byte(`
		<?xml version="1.0"?>
		<methodResponse>
		<params>
			<param>
				<value>` + toXml(ret, true) + `</value>
			</param>
		</params>
		</methodResponse>
		`))
	}
}

func TestAddInt(t *testing.T) {
	ts := httptest.NewServer(createServer("/api", "AddInt", func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			return nil, errors.New("bad number of arguments")
		}
		switch args[0].(type) {
		case int:
		default:
			return nil, errors.New("args[0] should be int")
		}
		switch args[1].(type) {
		case int:
		default:
			return nil, errors.New("args[1] should be int")
		}
		return args[0].(int) + args[1].(int), nil
	}))
	defer ts.Close()

	client := NewClient(ts.URL + "/api")
	v, err := client.Call("AddInt", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	i, ok := v.(int)
	if !ok {
		t.Fatalf("want int but got %T: %v", v, v)
	}
	if i != 3 {
		t.Fatalf("want %v but got %v", 3, v)
	}
}

func TestAddString(t *testing.T) {
	ts := httptest.NewServer(createServer("/api", "AddString", func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			return nil, errors.New("bad number of arguments")
		}
		switch args[0].(type) {
		case string:
		default:
			return nil, errors.New("args[0] should be string")
		}
		switch args[1].(type) {
		case string:
		default:
			return nil, errors.New("args[1] should be string")
		}
		return args[0].(string) + args[1].(string), nil
	}))
	defer ts.Close()

	client := NewClient(ts.URL + "/api")
	v, err := client.Call("AddString", "hello", "world")
	if err != nil {
		t.Fatal(err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("want string but got %T: %v", v, v)
	}
	if s != "helloworld" {
		t.Fatalf("want %q but got %q", "helloworld", v)
	}
}

type ParseStructArrayHandler struct {
}

func (h *ParseStructArrayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`
<?xml version="1.0"?>
<methodResponse>
  <params>
    <param>
      <value>
        <array>
          <data>
            <value>
              <struct>
                <member>
                  <name>test1</name>
                  <value>
                    <string>a</string>
                  </value>
                </member>
                <member>
                  <name>test2</name>
                  <value>
                    <int>2</int>
                  </value>
                </member>
              </struct>
            </value>
            <value>
              <struct>
                <member>
                  <name>test1</name>
                  <value>
                    <string>b</string>
                  </value>
                </member>
                <member>
                  <name>test2</name>
                  <value>
                    <int>2</int>
                  </value>
                </member>
              </struct>
            </value>
            <value>
              <struct>
                <member>
                  <name>test1</name>
                  <value>
                    <string>c</string>
                  </value>
                </member>
                <member>
                  <name>test2</name>
                  <value>
                    <int>2</int>
                  </value>
                </member>
              </struct>
            </value>
          </data>
        </array>
      </value>
    </param>
  </params>
</methodResponse>
		`))
}

func TestParseStructArray(t *testing.T) {
	ts := httptest.NewServer(&ParseStructArrayHandler{})
	defer ts.Close()

	res, err := NewClient(ts.URL + "/").Call("Irrelevant")

	if err != nil {
		t.Fatal(err)
	}

	if len(res.(Array)) != 3 {
		t.Fatal("expected array with 3 entries")
	}
}

type ParseIntArrayHandler struct {
}

func (h *ParseIntArrayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`
<?xml version="1.0"?>
<methodResponse>
  <params>
    <param>
      <value>
        <array>
          <data>
            <value>
              <int>2</int>
            </value>
            <value>
              <int>3</int>
            </value>
            <value>
              <int>4</int>
            </value>
          </data>
        </array>
      </value>
    </param>
  </params>
</methodResponse>
		`))
}

func TestParseIntArray(t *testing.T) {
	ts := httptest.NewServer(&ParseIntArrayHandler{})
	defer ts.Close()

	res, err := NewClient(ts.URL + "/").Call("Irrelevant")

	if err != nil {
		t.Fatal(err)
	}

	if len(res.(Array)) != 3 {
		t.Fatal("expected array with 3 entries")
	}
}

type ParseMixedArrayHandler struct {
}

func (h *ParseMixedArrayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`
<?xml version="1.0"?>
<methodResponse>
  <params>
    <param>
      <value>
        <array>
          <data>
            <value>
              <struct>
                <member>
                  <name>test1</name>
                  <value>
                    <string>a</string>
                  </value>
                </member>
                <member>
                  <name>test2</name>
                  <value>
                    <int>2</int>
                  </value>
                </member>
              </struct>
            </value>
            <value>
              <int>2</int>
            </value>
            <value>
              <struct>
                <member>
                  <name>test1</name>
                  <value>
                    <string>b</string>
                  </value>
                </member>
                <member>
                  <name>test2</name>
                  <value>
                    <int>2</int>
                  </value>
                </member>
              </struct>
            </value>
            <value>
              <int>4</int>
            </value>
          </data>
        </array>
      </value>
    </param>
  </params>
</methodResponse>
		`))
}

func TestParseMixedArray(t *testing.T) {
	ts := httptest.NewServer(&ParseMixedArrayHandler{})
	defer ts.Close()

	res, err := NewClient(ts.URL + "/").Call("Irrelevant")

	if err != nil {
		t.Fatal(err)
	}

	if len(res.(Array)) != 4 {
		t.Fatal("expected array with 4 entries")
	}
}
