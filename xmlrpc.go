package xmlrpc

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type Array []interface{}
type Struct map[string]interface{}

func next(p *xml.Decoder) (xml.Name, interface{}, error) {
	se, nextErr := nextStart(p)
	if nextErr != nil {
		return xml.Name{}, nil, nextErr
	}

	var nv interface{}
	switch se.Name.Local {
	case "string":
		var s string
		if e := p.DecodeElement(&s, &se); e != nil {
			return xml.Name{}, nil, e
		}
		return xml.Name{}, s, nil
	case "boolean":
		var s string
		if e := p.DecodeElement(&s, &se); e != nil {
			return xml.Name{}, nil, e
		}
		s = strings.TrimSpace(s)
		var b bool
		switch s {
		case "true", "1":
			b = true
		case "false", "0":
			b = false
		default:
			return xml.Name{}, b, errors.New("invalid boolean value")
		}
		return xml.Name{}, b, nil
	case "int", "i1", "i2", "i4", "i8":
		var s string
		var i int
		if e := p.DecodeElement(&s, &se); e != nil {
			return xml.Name{}, nil, e
		}
		i, e := strconv.Atoi(strings.TrimSpace(s))
		return xml.Name{}, i, e
	case "double":
		var s string
		var f float64
		if e := p.DecodeElement(&s, &se); e != nil {
			return xml.Name{}, nil, e
		}
		f, e := strconv.ParseFloat(strings.TrimSpace(s), 64)
		return xml.Name{}, f, e
	case "dateTime.iso8601":
		var s string
		if e := p.DecodeElement(&s, &se); e != nil {
			return xml.Name{}, nil, e
		}
		t, e := time.Parse("20060102T15:04:05", s)
		if e != nil {
			t, e = time.Parse("2006-01-02T15:04:05-07:00", s)
			if e != nil {
				t, e = time.Parse("2006-01-02T15:04:05", s)
			}
		}
		return xml.Name{}, t, e
	case "base64":
		var s string
		if e := p.DecodeElement(&s, &se); e != nil {
			return xml.Name{}, nil, e
		}
		if b, e := base64.StdEncoding.DecodeString(s); e != nil {
			return xml.Name{}, nil, e
		} else {
			return xml.Name{}, b, nil
		}
	case "member":
		nextStart(p)
		return next(p)

	case "value", "name", "param":
		nextStart(p)
		return next(p)

	case "struct":
		st := Struct{}

		var e error
		se, e = nextStart(p)
		for e == nil && se.Name.Local == "member" {
			// name
			se, e = nextStart(p)
			if se.Name.Local != "name" {
				return xml.Name{}, nil, errors.New("invalid response")
			}
			if e != nil {
				break
			}
			var name string
			if e = p.DecodeElement(&name, &se); e != nil {
				return xml.Name{}, nil, e
			}
			se, e = nextStart(p)
			if e != nil {
				break
			}

			// value
			_, value, e := next(p)
			if se.Name.Local != "value" {
				return xml.Name{}, nil, errors.New("invalid response")
			}
			if e != nil {
				break
			}
			st[name] = value

			se, e = nextStart(p)
			if e != nil {
				break
			}
		}
		return xml.Name{}, st, e

	case "array":
		var ar Array
		nextStart(p) // data
		for {
			nextStart(p) // top of value
			_, value, e := next(p)
			if e != nil {
				break
			}
			ar = append(ar, value)
		}
		return xml.Name{}, ar, nil

	case "nil":
		return xml.Name{}, nil, nil

	case "params":
		var ar Array
		for {
			_, value, e := next(p)
			if e != nil {
				break
			}
			ar = append(ar, value)
		}
		return xml.Name{}, ar, nil

	case "fault":
		_, value, _ := next(p)
		fs, ok := value.(Struct)
		if !ok {
			return xml.Name{}, value, fmt.Errorf("fault: wanted Struct, got %#v", value)
		}
		var f Fault
		if s, ok := fs["faultCode"].(string); ok {
			f.Code, _ = strconv.Atoi(s)
		}
		f.Message, _ = fs["faultString"].(string)
		return xml.Name{}, nil, &f

	}

	if e := p.DecodeElement(&nv, &se); e != nil {
		return xml.Name{}, nil, e
	}
	return se.Name, nv, nil
}
func nextStart(p *xml.Decoder) (xml.StartElement, error) {
	for {
		t, e := p.Token()
		if e != nil {
			return xml.StartElement{}, e
		}
		switch t := t.(type) {
		case xml.StartElement:
			return t, nil
		}
	}
}

var UnsupportedType = errors.New("unsupported type")

func writeXML(w io.Writer, v interface{}, typ bool) error {
	if v == nil {
		_, err := io.WriteString(w, "<nil/>")
		return err
	}
	r := reflect.ValueOf(v)
	t := r.Type()
	k := t.Kind()

	if b, ok := v.([]byte); ok {
		io.WriteString(w, "<base64>")
		_, err := base64.NewEncoder(base64.StdEncoding, w).Write(b)
		io.WriteString(w, "</base64>")
		return err
	}

	switch k {
	case reflect.Invalid:
		return UnsupportedType
	case reflect.Bool:
		_, err := fmt.Fprintf(w, "<boolean>%v</boolean>", v)
		return err
	case reflect.Int,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if typ {
			_, err := fmt.Fprintf(w, "<int>%v</int>", v)
			return err
		}
		_, err := fmt.Fprintf(w, "%v", v)
		return err
	case reflect.Uintptr:
		return UnsupportedType
	case reflect.Float32, reflect.Float64:
		if typ {
			_, err := fmt.Fprintf(w, "<double>%v</double>", v)
			return err
		}
		_, err := fmt.Fprintf(w, "%v", v)
		return err
	case reflect.Complex64, reflect.Complex128:
		return UnsupportedType
	case reflect.Array:
		io.WriteString(w, "<array><data>")
		for n := 0; n < r.Len(); n++ {
			io.WriteString(w, "<value>")
			err := writeXML(w, r.Index(n).Interface(), typ)
			io.WriteString(w, "</value>")
			if err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, "</data></array>")
		return err
	case reflect.Chan:
		return UnsupportedType
	case reflect.Func:
		return UnsupportedType
	case reflect.Interface:
		return writeXML(w, r.Elem(), typ)
	case reflect.Map:
		io.WriteString(w, "<struct>")
		for _, key := range r.MapKeys() {
			io.WriteString(w, "<member><name>")
			if err := xml.EscapeText(w, []byte(key.Interface().(string))); err != nil {
				return err
			}
			io.WriteString(w, "</name><value>")
			if err := writeXML(w, r.MapIndex(key).Interface(), typ); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "</value></member>"); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, "</struct>")
		return err
	case reflect.Ptr:
		return UnsupportedType
	case reflect.Slice:
		return UnsupportedType
	case reflect.String:
		if typ {
			io.WriteString(w, "<string>")
		}
		err := xml.EscapeText(w, []byte(v.(string)))
		if typ {
			io.WriteString(w, "</string>")
		}
		return err
	case reflect.Struct:
		io.WriteString(w, "<struct>")
		for n := 0; n < r.NumField(); n++ {
			fmt.Fprintf(w, "<member><name>%s</name><value>", t.Field(n).Name)
			if err := writeXML(w, r.FieldByIndex([]int{n}).Interface(), true); err != nil {
				return err
			}
			io.WriteString(w, "</value></member>")
		}
		_, err := io.WriteString(w, "</struct>")
		return err
	case reflect.UnsafePointer:
		return writeXML(w, r.Elem(), typ)
	}
	return nil
}

// Client is client of XMLRPC
type Client struct {
	HttpClient *http.Client
	url        string
}

// NewClient create new Client
func NewClient(url string) *Client {
	return &Client{
		HttpClient: &http.Client{Transport: http.DefaultTransport, Timeout: 10 * time.Second},
		url:        url,
	}
}

func Marshal(w io.Writer, name string, args ...interface{}) error {
	io.WriteString(w, `<?xml version="1.0"?>`)
	var end string
	if name == "" {
		io.WriteString(w, "<methodResponse>")
		end = "</methodResponse>"
	} else {
		io.WriteString(w, "<methodCall><methodName>")
		if err := xml.EscapeText(w, []byte(name)); err != nil {
			return err
		}
		io.WriteString(w, "</methodName>")
		end = "</methodCall>"
	}

	io.WriteString(w, "<params>")
	for _, arg := range args {
		io.WriteString(w, "<param><value>")
		if err := writeXML(w, arg, true); err != nil {
			return err
		}
		io.WriteString(w, "</value></param>")
	}
	io.WriteString(w, "</params>")
	_, err := io.WriteString(w, end)
	return err
}

func makeRequest(name string, args ...interface{}) *bytes.Buffer {
	var buf bytes.Buffer
	if err := Marshal(&buf, name, args...); err != nil {
		panic(err)
	}
	return &buf
}
func call(client *http.Client, url, name string, args ...interface{}) (v Array, e error) {
	r, e := http.DefaultClient.Post(url, "text/xml", makeRequest(name, args...))
	if e != nil {
		return nil, e
	}

	// Since we do not always read the entire body, discard the rest, which
	// allows the http transport to reuse the connection.
	defer io.Copy(ioutil.Discard, r.Body)
	defer r.Body.Close()

	if r.StatusCode/100 != 2 {
		return nil, errors.New(http.StatusText(http.StatusBadRequest))
	}

	_, v, e = Unmarshal(r.Body)
	return v, e
}

func Unmarshal(r io.Reader) (string, Array, error) {
	var name string
	p := xml.NewDecoder(r)
	se, e := nextStart(p) // methodResponse
	if e != nil {
		return name, nil, e
	}
	if se.Name.Local != "methodResponse" {
		if se.Name.Local != "methodCall" {
			return name, nil, errors.New("invalid response: missing methodResponse")
		}
		if se, e = nextStart(p); e != nil {
			return name, nil, e
		}
		if se.Name.Local != "methodName" {
			return name, nil, errors.New("invalid response: missing methodName")
		}
		if e = p.DecodeElement(&name, &se); e != nil {
			return name, nil, e
		}
	}
	_, v, e := next(p)
	if a, ok := v.(Array); ok {
		return name, a, e
	} else if e == nil {
		e = fmt.Errorf("wanted Array, got %#v", v)
	}
	return name, nil, e
}

type Fault struct {
	Code    int
	Message string
}

func (f *Fault) Error() string { return fmt.Sprintf("%d: %s", f.Code, f.Message) }

// Call call remote procedures function name with args
func (c *Client) Call(name string, args ...interface{}) (v Array, e error) {
	return call(c.HttpClient, c.url, name, args...)
}

// Call call remote procedures function name with args
func Call(url, name string, args ...interface{}) (v Array, e error) {
	return call(http.DefaultClient, url, name, args...)
}
