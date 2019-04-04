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
			v, err := next(p)
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

type ParseNestedArray struct {
}

func (h *ParseNestedArray) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`
	<?xml version="1.0" encoding="UTF-8"?>
	<methodResponse>
	<params>
	<param>
	<value><array>
	<data>
	<value><array>
	<data>
	<value><array>
	<data>
	<value><struct>
	<member><name>type</name>
	<value><string>folder</string></value>
	</member>
	<member><name>folderid</name>
	<value><string>QVlJS3ZXTjGu4lczs4ugVw</string></value>
	</member>
	<member><name>label</name>
	<value><string>SEJOURS</string></value>
	</member>
	<member><name>modificationdate</name>
	<value><double>0</double></value>
	</member>
	<member><name>createddate</name>
	<value><double>1554311194000</double></value>
	</member>
	</struct></value>
	<value><struct>
	<member><name>type</name>
	<value><string>folder</string></value>
	</member>
	<member><name>folderid</name>
	<value><string>sdsd</string></value>
	</member>
	<member><name>label</name>
	<value><string>ETE</string></value>
	</member>
	<member><name>modificationdate</name>
	<value><double>0</double></value>
	</member>
	<member><name>createddate</name>
	<value><double>1554221058000</double></value>
	</member>
	</struct></value>
	</data>
	</array></value>
	</data>
	</array></value>
	<value><array>
	<data>
	<value><array>
	<data>
	<value><struct>
	<member><name>albumid</name>
	<value><string>QVlJS3ZXTjEpIPrMXDKLuw</string></value>
	</member>
	<member><name>label</name>
	<value><string>PHOTOS</string></value>
	</member>
	<member><name>orderby</name>
	<value><string>date</string></value>
	</member>
	<member><name>ordertype</name>
	<value><string>ascending</string></value>
	</member>
	<member><name>date</name>
	<value><double>1554221064000</double></value>
	</member>
	<member><name>createddate</name>
	<value><double>1554221068000</double></value>
	</member>
	<member><name>lastvisitdate</name>
	<value><double>1554221068000</double></value>
	</member>
	<member><name>lastfileaddeddate</name>
	<value><double>0</double></value>
	</member>
	<member><name>public</name>
	<value><int>0</int></value>
	</member>
	<member><name>allowdownload</name>
	<value><int>1</int></value>
	</member>
	<member><name>allowupload</name>
	<value><int>0</int></value>
	</member>
	<member><name>allowprintorder</name>
	<value><int>1</int></value>
	</member>
	<member><name>allowsendcomments</name>
	<value><int>1</int></value>
	</member>
	<member><name>folderid</name>
	<value><string>zezeze</string></value>
	</member>
	</struct></value>
	<value><struct>
	<member><name>albumid</name>
	<value><string>zeeze</string></value>
	</member>
	<member><name>label</name>
	<value><string>test</string></value>
	</member>
	<member><name>orderby</name>
	<value><string>date</string></value>
	</member>
	<member><name>ordertype</name>
	<value><string>ascending</string></value>
	</member>
	<member><name>date</name>
	<value><double>1554311214000</double></value>
	</member>
	<member><name>createddate</name>
	<value><double>1554311217000</double></value>
	</member>
	<member><name>lastvisitdate</name>
	<value><double>1554311241000</double></value>
	</member>
	<member><name>lastfileaddeddate</name>
	<value><double>1554311238000</double></value>
	</member>
	<member><name>public</name>
	<value><int>0</int></value>
	</member>
	<member><name>allowdownload</name>
	<value><int>1</int></value>
	</member>
	<member><name>allowupload</name>
	<value><int>0</int></value>
	</member>
	<member><name>allowprintorder</name>
	<value><int>1</int></value>
	</member>
	<member><name>allowsendcomments</name>
	<value><int>1</int></value>
	</member>
	<member><name>folderid</name>
	<value><string>QVlJS3ZXTjGM3tkbWu_Z0A</string></value>
	</member>
	</struct></value>
	</data>
	</array></value>
	</data>
	</array></value>
	<value><array>
	<data>
	<value><array>
	<data>
	<value><struct>
	<member><name>contactid</name>
	<value><string>QVlJS3ZXzeeeTjH9d-QUwXR5jg</string></value>
	</member>
	<member><name>login</name>
	<value><string>benoit.zez</string></value>
	</member>
	<member><name>type</name>
	<value><string>0</string></value>
	</member>
	<member><name>usePreferences</name>
	<value><string>0</string></value>
	</member>
	<member><name>password</name>
	<value><string>ddere</string></value>
	</member>
	<member><name>firstname</name>
	<value><string></string></value>
	</member>
	<member><name>lastname</name>
	<value><string></string></value>
	</member>
	<member><name>email</name>
	<value><string>zelkzjez@test.fr</string></value>
	</member>
	<member><name>phoneNumber</name>
	<value><string></string></value>
	</member>
	</struct></value>
	<value><struct>
	<member><name>contactid</name>
	<value><string>zezez</string></value>
	</member>
	<member><name>login</name>
	<value><string>zezezeze</string></value>
	</member>
	<member><name>type</name>
	<value><string>0</string></value>
	</member>
	<member><name>usePreferences</name>
	<value><string>0</string></value>
	</member>
	<member><name>password</name>
	<value><string>dezzrere</string></value>
	</member>
	<member><name>firstname</name>
	<value><string>a</string></value>
	</member>
	<member><name>lastname</name>
	<value><string>a</string></value>
	</member>
	<member><name>email</name>
	<value><string>xxxxx@gmail.com</string></value>
	</member>
	<member><name>phoneNumber</name>
	<value><string></string></value>
	</member>
	</struct></value>
	<value><struct>
	<member><name>contactid</name>
	<value><string>QVlJS3ZXTjddFSflbF5i2KMQ</string></value>
	</member>
	<member><name>login</name>
	<value><string>eee</string></value>
	</member>
	<member><name>type</name>
	<value><string>0</string></value>
	</member>
	<member><name>usePreferences</name>
	<value><string>0</string></value>
	</member>
	<member><name>password</name>
	<value><string>eeeee</string></value>
	</member>
	<member><name>firstname</name>
	<value><string>Benoit</string></value>
	</member>
	<member><name>lastname</name>
	<value><string>KUG</string></value>
	</member>
	<member><name>email</name>
	<value><string>sderedr@free.fr</string></value>
	</member>
	<member><name>phoneNumber</name>
	<value><string></string></value>
	</member>
	</struct></value>
	</data>
	</array></value>
	</data>
	</array></value>
	</data>
	</array></value>
	</param>
	</params>
	</methodResponse>`))
}

func TestParseNestedArray(t *testing.T) {
	ts := httptest.NewServer(&ParseNestedArray{})
	defer ts.Close()

	res, err := NewClient(ts.URL + "/").Call("Irrelevant")

	if err != nil {
		t.Fatal(err)
	}

	if len(res.(Array)) != 3 {
		t.Fatal("expected array with 3 entries")
	}
}
