package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	// "github.com/aws/aws-sdk-go/aws"
	// "github.com/aws/aws-sdk-go/aws/session"
	// "github.com/aws/aws-sdk-go/service/kinesis"
)

var (
	port   = flag.String("port", "8000", "listening port")
	target = flag.String("target", "localhost:9000", "proxy target")
)

// // Firehose implements AWS Kinesis writer
// type Firehose struct {
// 	session *session.Session
// 	kinesis *kinesis.Kinesis
// }

// func (f *Firehose) Write(data []byte) (int, error) {
// 	record := &kinesis.PutRecordInput{
// 		Data:         data,
// 		PartitionKey: aws.String(""),
// 		StreamName:   aws.String(""),
// 	}

// 	_, err := f.kinesis.PutRecord(record)
// 	return len(data), err
// }

var reqWriteExcludeHeaderDump = map[string]bool{
	"Host":              true, // not in Header map anyway
	"Transfer-Encoding": true,
	"Trailer":           true,
}

// DumpRequest returns the given request based on httputils.DumpRequest.
func DumpRequest(req *http.Request) ([]byte, error) {
	var b bytes.Buffer

	reqURI := req.RequestURI
	if reqURI == "" {
		reqURI = req.URL.RequestURI()
	}

	method := req.Method
	if method == "" {
		method = "GET"
	}

	fmt.Fprintf(&b, "%s %s HTTP/%d.%d\r\n", method, reqURI, req.ProtoMajor, req.ProtoMinor)

	absRequestURI := strings.HasPrefix(req.RequestURI, "http://") || strings.HasPrefix(req.RequestURI, "https://")
	if !absRequestURI {
		host := req.Host
		if host == "" && req.URL != nil {
			host = req.URL.Host
		}
		if host != "" {
			fmt.Fprintf(&b, "Host: %s\r\n", host)
		}
	}

	chunked := len(req.TransferEncoding) > 0 && req.TransferEncoding[0] == "chunked"
	if len(req.TransferEncoding) > 0 {
		fmt.Fprintf(&b, "Transfer-Encoding: %s\r\n", strings.Join(req.TransferEncoding, ","))
	}
	if req.Close {
		fmt.Fprintf(&b, "Connection: close\r\n")
	}

	err := req.Header.WriteSubset(&b, reqWriteExcludeHeaderDump)
	if err != nil {
		return nil, err
	}

	io.WriteString(&b, "\r\n")

	if req.Body != nil {
		var save bytes.Buffer
		var body io.Writer = &save
		if chunked {
			body = httputil.NewChunkedWriter(body)
		}

		if _, err := save.ReadFrom(req.Body); err != nil {
			return nil, err
		}
		if err := req.Body.Close(); err != nil {
			return nil, err
		}
		req.Body = ioutil.NopCloser(bytes.NewBuffer(save.Bytes()))
		if _, err := io.Copy(&b, &save); err != nil {
			return nil, err
		}
		if chunked {
			body.(io.Closer).Close()
			io.WriteString(&b, "\r\n")
		}
	}

	return b.Bytes(), nil
}

// DumpResponse like DumpRequest based on httputils.DumpResonse.
func DumpResponse(resp *http.Response) ([]byte, error) {
	var b bytes.Buffer

	var save bytes.Buffer
	savecl := resp.ContentLength
	if resp.ContentLength != 0 {
		if _, err := io.Copy(&save, resp.Body); err != nil {
			return nil, err
		}
	}

	err := resp.Write(&b)
	resp.Body = ioutil.NopCloser(&save)
	resp.ContentLength = savecl
	return b.Bytes(), err
}

// DumpRoundTripper implements http.RoundTripper interface for
// a wrapped http.RoundTripper dumping the request and response.
type DumpRoundTripper struct {
	Transport http.RoundTripper
	Stream    io.Writer
}

func (d *DumpRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	b1, err := DumpRequest(req)
	if err != nil {
		return nil, err
	}
	if _, err := d.Stream.Write(b1); err != nil {
		return nil, err
	}

	resp, err := d.Transport.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	b2, err := DumpResponse(resp)
	if err != nil {
		return nil, err
	}
	if _, err := d.Stream.Write(b2); err != nil {
		return nil, err
	}
	return resp, nil
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

func NewDumpReverseProxy(target *url.URL, transport http.RoundTripper, stream io.Writer) *httputil.ReverseProxy {
	targetQuery := target.RawQuery

	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)
			if targetQuery == "" || req.URL.RawQuery == "" {
				req.URL.RawQuery = targetQuery + req.URL.RawQuery
			} else {
				req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
			}
			if _, ok := req.Header["User-Agent"]; !ok {
				// explicitly disable User-Agent so it's not set to default value
				req.Header.Set("User-Agent", "")
			}
		},
		Transport: &DumpRoundTripper{
			Transport: transport,
			Stream:    stream,
		},
	}
}

func init() {
	flag.Parse()
}

func main() {
	proxy := NewDumpReverseProxy(&url.URL{
		Scheme: "http",
		Host:   *target,
	}, http.DefaultTransport, os.Stdout)

	log.Println("Running...")
	http.ListenAndServe(":"+*port, proxy)
}
