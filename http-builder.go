package lets

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/google/go-querystring/query"
)

// Type for saving header value.
type httpHeader struct {
	Name  string
	Value string
}

type basicAuth struct {
	Username string
	Password string
}

// Type for saving oy builder params.
type HttpBuilder struct {
	url       string
	client    *http.Client
	headers   []*httpHeader
	response  *http.Response
	basic     basicAuth
	multipart bool
}

type HttpBuilderOptions struct {
	LogHeader      bool
	LogMethod      bool
	LogRequestBody bool
	LogResponse    bool
}

// Set http client with default configuration.
func (h *HttpBuilder) Default() {
	defaultTransport := http.DefaultTransport.(*http.Transport)

	// Create new Transport that ignores self-signed SSL
	customTransport := &http.Transport{
		Proxy:                 defaultTransport.Proxy,
		DialContext:           defaultTransport.DialContext,
		MaxIdleConns:          defaultTransport.MaxIdleConns,
		IdleConnTimeout:       defaultTransport.IdleConnTimeout,
		ExpectContinueTimeout: defaultTransport.ExpectContinueTimeout,
		TLSHandshakeTimeout:   defaultTransport.TLSHandshakeTimeout,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
	}

	h.client = &http.Client{
		Timeout:   time.Duration(10) * time.Second,
		Transport: customTransport,
	}
}

func (h *HttpBuilder) MultipartEnable() {
	h.multipart = true
}

func (h *HttpBuilder) UseCookie(jar *cookiejar.Jar) error {
	if jar == nil {
		var err error
		jar, err = cookiejar.New(
			&cookiejar.Options{},
		)
		if err != nil {
			return err
		}
	}

	h.client.Jar = jar

	return nil
}

func (h *HttpBuilder) GetCookie() (jar http.CookieJar) {
	return h.client.Jar
}

func (h *HttpBuilder) UseProxy(proxyServer string) error {
	transport := h.client.Transport.(*http.Transport)
	proxyUrl, err := url.Parse(proxyServer)
	if err != nil {
		return err
	}
	transport.Proxy = http.ProxyURL(proxyUrl)

	return nil
}

// Manual set http builder.
func (h *HttpBuilder) SetClient(client *http.Client) {
	h.client = client
}

// Set End Point
func (h *HttpBuilder) SetUrl(url string) {
	h.url = url
}

// Set Basic Auth
func (h *HttpBuilder) SetBasicAuth(username, password string) {
	h.basic.Username = username
	h.basic.Password = password
}

// Setting up header name and value.
func (h *HttpBuilder) AddHeader(name string, value string) {
	for _, header := range h.headers {
		if header.Name == name {
			header.Value = value
			return
		}
	}

	h.headers = append(h.headers, &httpHeader{
		Name:  name,
		Value: value,
	})
}

// Post request.
func (h *HttpBuilder) Post(endPoint string, body interface{}, option HttpBuilderOptions) (fullUrl string, responseStatusCode int, responseBody string, err error) {
	fullUrl = fmt.Sprintf("%s%s", h.url, endPoint)

	var payload, payloadDebug io.Reader

	if reflect.TypeOf(body) == reflect.TypeOf([]byte{}) {
		payload = strings.NewReader(string(body.([]byte)))
		payloadDebug = strings.NewReader(string(body.([]byte)))
	} else if reflect.TypeOf(body) == reflect.PointerTo(reflect.TypeOf(bytes.Buffer{})) {
		payload = body.(*bytes.Buffer)
		payloadDebug = body.(*bytes.Buffer)
	} else {
		payload = strings.NewReader(ToJson(body))
		payloadDebug = strings.NewReader(ToJson(body))
	}

	if option.LogMethod {
		LogI("HttpBuilder: POST \"%s\"\n", fullUrl)
	}

	req, err := http.NewRequest(http.MethodPost, fullUrl, payload)
	if err != nil {
		return
	}

	// Header Setup
	for _, header := range h.headers {
		req.Header.Add(header.Name, header.Value)

		if option.LogHeader {
			LogI("HttpBuilder: SetHeader: %s: %s", header.Name, header.Value)
		}
	}

	if option.LogRequestBody {
		// Get Body
		var body []byte
		if payloadDebug != nil {
			body, err = io.ReadAll(payloadDebug)
			if err != nil {
				return
			}
			// Assign Back the request body
			payloadDebug = io.NopCloser(bytes.NewBuffer(body))
			LogI("HttpBuilder: Request Body:\n%s\n", string(body))
		}
	}

	// Basic Auth
	if h.basic.Username != "" {
		req.SetBasicAuth(h.basic.Username, h.basic.Password)
	}

	h.response, err = h.client.Do(req)
	if err != nil {
		return
	}
	defer h.response.Body.Close()

	resBody, err := io.ReadAll(h.response.Body)
	if err != nil {
		return
	}

	responseStatusCode = h.response.StatusCode
	responseBody = string(resBody)

	if option.LogResponse {
		LogI("HttpBuilder: Response Status: %v", h.response.StatusCode)
		LogI("HttpBuilder: Response Body: %s\n\n", responseBody)
	}

	return
}

// Get request.
func (h *HttpBuilder) Get(endPoint string, urlQuery interface{}, body interface{}, option HttpBuilderOptions) (fullUrl string, responseStatusCode int, responseBody string, err error) {
	fullUrl = fmt.Sprintf("%s%s", h.url, endPoint)

	// Process Query
	if reflect.TypeOf(urlQuery) == reflect.TypeOf([]byte(nil)) {
		fullUrl = fmt.Sprintf("%s?%s", fullUrl, string(urlQuery.([]byte)))
	} else if urlQuery != nil {
		v, _ := query.Values(urlQuery)
		fullUrl = fmt.Sprintf("%s?%s", fullUrl, v.Encode())
	}

	// Process Body
	var payload io.Reader
	if reflect.TypeOf(body) == reflect.TypeOf([]byte(nil)) {
		payload = strings.NewReader(string(body.([]byte)))
	} else if reflect.TypeOf(body) == reflect.PointerTo(reflect.TypeOf(bytes.Buffer{})) {
		payload = body.(*bytes.Buffer)
	} else if reflect.TypeOf(body) == reflect.TypeOf(string("")) {
		payload = strings.NewReader(ToJson(body))
	} else {
		payload = &strings.Reader{}
	}

	if option.LogMethod {
		LogI("HttpBuilder: GET \"%s\"\n", fullUrl)
	}

	req, err := http.NewRequest(http.MethodGet, fullUrl, payload)
	if err != nil {
		return
	}

	// Header Setup
	for _, header := range h.headers {
		req.Header.Add(header.Name, header.Value)

		if option.LogHeader {
			LogI("HttpBuilder: SetHeader: %s: %s", header.Name, header.Value)
		}
	}

	if option.LogRequestBody {
		body, _ := io.ReadAll(payload)
		LogI("HttpBuilder: Body:\n%s\n", string(body))
	}

	h.response, err = h.client.Do(req)
	if err != nil {
		return
	}
	defer h.response.Body.Close()

	resBody, err := io.ReadAll(h.response.Body)
	if err != nil {
		return
	}

	responseStatusCode = h.response.StatusCode
	responseBody = string(resBody)

	if option.LogResponse {
		LogI("HttpBuilder: Response Status: %v", h.response.StatusCode)
		LogI("HttpBuilder: Response Body: %s\n\n", responseBody)
	}
	return
}

func (h *HttpBuilder) GetRequest() *http.Response {
	return h.response
}

func (h *HttpBuilder) GetResponse() *http.Response {
	return h.response
}
