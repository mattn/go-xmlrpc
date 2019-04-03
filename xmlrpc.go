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

// Array is a generic array
type Array []interface{}

// Struct is a generic map
type Struct map[string]interface{}

var (
	xmlSpecial = map[byte]string{
		'<':  "&lt;",
		'>':  "&gt;",
		'"':  "&quot;",
		'\'': "&apos;",
		'&':  "&amp;",
	}
	errStartingTagNotFound = errors.New("starting tag expexted")
	errEndingTagNotFound   = errors.New("ending tag expexted")
)

func xmlEscape(s string) string {
	var b bytes.Buffer
	for i := 0; i < len(s); i++ {
		c := s[i]
		if s, ok := xmlSpecial[c]; ok {
			b.WriteString(s)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func next(p *xml.Decoder) (interface{}, error) {
	se, e := nextStart(p)
	if e != nil {
		return nil, e
	}
	switch se.Name.Local {
	case "string":
		var s string
		if e = p.DecodeElement(&s, &se); e != nil {
			return nil, e
		}
		return s, nil
	case "boolean":
		var s string
		if e = p.DecodeElement(&s, &se); e != nil {
			return nil, e
		}
		s = strings.TrimSpace(s)
		var b bool
		switch s {
		case "true", "1":
			b = true
		case "false", "0":
			b = false
		default:
			e = errors.New("invalid boolean value")
		}
		return b, e
	case "int", "i1", "i2", "i4", "i8":
		var s string
		var i int
		if e = p.DecodeElement(&s, &se); e != nil {
			return nil, e
		}
		i, e = strconv.Atoi(strings.TrimSpace(s))
		return i, e
	case "double":
		var s string
		var f float64
		if e = p.DecodeElement(&s, &se); e != nil {
			return nil, e
		}
		f, e = strconv.ParseFloat(strings.TrimSpace(s), 64)
		return f, e
	case "dateTime.iso8601":
		var s string
		if e = p.DecodeElement(&s, &se); e != nil {
			return nil, e
		}
		t, e := time.Parse("20060102T15:04:05", s)
		if e != nil {
			t, e = time.Parse("2006-01-02T15:04:05-07:00", s)
			if e != nil {
				t, e = time.Parse("2006-01-02T15:04:05", s)
			}
		}
		return t, e
	case "base64":
		var s string
		if e = p.DecodeElement(&s, &se); e != nil {
			return nil, e
		}
		b, e := base64.StdEncoding.DecodeString(s)
		if e != nil {
			return nil, e
		}
		return b, nil
	case "struct":
		st := Struct{}

		for {
			se, err := nextStart(p)
			if err == errStartingTagNotFound { // end of struct
				break
			} else if err != nil {
				return nil, err
			} else if se.Name.Local != "member" {
				return nil, errors.New("member element expected")
			}
			se, err = nextStart(p)
			if err != nil {
				return nil, err
			} else if se.Name.Local != "name" {
				return nil, errors.New("name element expected")
			}
			var name string
			if e = p.DecodeElement(&name, &se); e != nil { // DecodeElement closes name element
				return nil, e
			}
			se, err = nextStart(p)
			if err != nil {
				return nil, err
			} else if se.Name.Local != "value" {
				return nil, errors.New("value element for member expected")
			}
			// value
			value, e := next(p)
			if e != nil {
				return nil, e
			}
			err = nextEnd(p) // value end
			if err != nil {
				return nil, err
			}
			err = nextEnd(p) // member end
			if err != nil {
				return nil, err
			}
			st[name] = value
		}
		return st, nil
	case "array":
		fmt.Println("reading array")
		se, err := nextStart(p) // data
		if err != nil {
			return nil, err
		} else if se.Name.Local != "data" {
			return nil, errors.New("data element expected")
		}
		var ar Array
		for {
			se, err := nextStart(p)            // value
			if err == errStartingTagNotFound { // end of array, end data reached
				break
			} else if err != nil {
				return nil, err
			} else if se.Name.Local != "value" {
				return nil, errors.New("value element expected")
			}
			value, e := next(p)
			if e != nil {
				return nil, e
			}
			err = nextEnd(p) // closing value
			if err != nil {
				return nil, err
			}

			ar = append(ar, value)

		}
		err = nextEnd(p) // closing array
		if err != nil {
			return nil, err
		}
		return ar, nil
	case "nil":
		return nil, nil
	default:
		var nv interface{}
		if e = p.DecodeElement(nv, &se); e != nil {
			return nil, e
		}
		return nv, e
	}
}

// func nextStart(p *xml.Decoder) (xml.StartElement, error) {
// 	for {
// 		t, e := p.Token()
// 		if e != nil {
// 			return xml.StartElement{}, e
// 		}
// 		switch t := t.(type) {
// 		case xml.StartElement:
// 			fmt.Println("at ", t.Name.Local)
// 			return t, nil
// 		case xml.EndElement:
// 			fmt.Println("closing ", t.Name.Local)
// 		}
// 	}
// }

func nextStart(p *xml.Decoder) (xml.StartElement, error) {
	for {
		t, e := p.Token()
		if e != nil {
			return xml.StartElement{}, e
		}
		switch t := t.(type) {
		case xml.StartElement:
			return t, nil
		case xml.EndElement:
			return xml.StartElement{}, errStartingTagNotFound
		}
	}
}

func nextEnd(p *xml.Decoder) error {
	for {
		t, e := p.Token()
		if e != nil {
			return e
		}
		switch t.(type) {
		case xml.EndElement:
			return nil
		case xml.StartElement:
			return errEndingTagNotFound
		}
	}
}

func toXml(v interface{}, typ bool) (s string) {
	if v == nil {
		return "<nil/>"
	}
	r := reflect.ValueOf(v)
	t := r.Type()
	k := t.Kind()

	if b, ok := v.([]byte); ok {
		return "<base64>" + base64.StdEncoding.EncodeToString(b) + "</base64>"
	}

	switch k {
	case reflect.Invalid:
		panic("unsupported type")
	case reflect.Bool:
		return fmt.Sprintf("<boolean>%v</boolean>", v)
	case reflect.Int,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if typ {
			return fmt.Sprintf("<int>%v</int>", v)
		}
		return fmt.Sprintf("%v", v)
	case reflect.Uintptr:
		panic("unsupported type")
	case reflect.Float32, reflect.Float64:
		if typ {
			return fmt.Sprintf("<double>%v</double>", v)
		}
		return fmt.Sprintf("%v", v)
	case reflect.Complex64, reflect.Complex128:
		panic("unsupported type")
	case reflect.Array:
		s = "<array><data>"
		for n := 0; n < r.Len(); n++ {
			s += "<value>"
			s += toXml(r.Index(n).Interface(), typ)
			s += "</value>"
		}
		s += "</data></array>"
		return s
	case reflect.Chan:
		panic("unsupported type")
	case reflect.Func:
		panic("unsupported type")
	case reflect.Interface:
		return toXml(r.Elem(), typ)
	case reflect.Map:
		s = "<struct>"
		for _, key := range r.MapKeys() {
			s += "<member>"
			s += "<name>" + xmlEscape(key.Interface().(string)) + "</name>"
			s += "<value>" + toXml(r.MapIndex(key).Interface(), typ) + "</value>"
			s += "</member>"
		}
		s += "</struct>"
		return s
	case reflect.Ptr:
		panic("unsupported type")
	case reflect.Slice:
		s = "<array><data>"
		for n := 0; n < r.Len(); n++ {
			s += "<value>"
			s += toXml(r.Index(n).Interface(), typ)
			s += "</value>"
		}
		s += "</data></array>"
		return s
	case reflect.String:
		if typ {
			return fmt.Sprintf("<string>%v</string>", xmlEscape(v.(string)))
		}
		return xmlEscape(v.(string))
	case reflect.Struct:
		s = "<struct>"
		for n := 0; n < r.NumField(); n++ {
			s += "<member>"
			s += "<name>" + t.Field(n).Name + "</name>"
			s += "<value>" + toXml(r.FieldByIndex([]int{n}).Interface(), true) + "</value>"
			s += "</member>"
		}
		s += "</struct>"
		return s
	case reflect.UnsafePointer:
		return toXml(r.Elem(), typ)
	}
	return
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

func makeRequest(name string, args ...interface{}) *bytes.Buffer {
	buf := new(bytes.Buffer)
	buf.WriteString(`<?xml version="1.0"?><methodCall>`)
	buf.WriteString("<methodName>" + xmlEscape(name) + "</methodName>")
	buf.WriteString("<params>")
	for _, arg := range args {
		buf.WriteString("<param><value>")
		buf.WriteString(toXml(arg, true))
		buf.WriteString("</value></param>")
	}
	buf.WriteString("</params></methodCall>")
	return buf
}

func call(client *http.Client, url, name string, args ...interface{}) (v interface{}, e error) {
	r, e := client.Post(url, "text/xml", makeRequest(name, args...))
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

	p := xml.NewDecoder(r.Body)
	se, e := nextStart(p) // methodResponse
	if se.Name.Local != "methodResponse" {
		return nil, errors.New("invalid response: missing methodResponse")
	}
	se, e = nextStart(p) // params
	if se.Name.Local != "params" {
		return nil, errors.New("invalid response: missing params")
	}
	se, e = nextStart(p) // param
	if se.Name.Local != "param" {
		return nil, errors.New("invalid response: missing param")
	}
	se, e = nextStart(p) // value
	if se.Name.Local != "value" {
		return nil, errors.New("invalid response: missing value")
	}
	v, e = next(p)
	return v, e
}

// Call call remote procedures function name with args
func (c *Client) Call(name string, args ...interface{}) (v interface{}, e error) {
	return call(c.HttpClient, c.url, name, args...)
}

// Global httpClient allows us to pool/reuse connections and not wastefully
// re-create transports for each request.
var httpClient = &http.Client{Transport: http.DefaultTransport, Timeout: 10 * time.Second}

// Call call remote procedures function name with args
func Call(url, name string, args ...interface{}) (v interface{}, e error) {
	return call(httpClient, url, name, args...)
}
