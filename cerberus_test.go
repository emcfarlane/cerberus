package cerberus

import (
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"strings"
	"testing"
)

func TestDumpRequest(t *testing.T) {
	req, err := http.NewRequest("POST", "http://example.com", strings.NewReader("{'hello': 'world!'}"))
	if err != nil {
		t.Fatal(err)
	}

	b1, err := httputil.DumpRequest(req, true)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := DumpRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(b1))
	t.Log(string(b2))

	for i := range b1 {
		if b1[i] != b2[i] {
			t.Fatalf("Mismatch at %d: %v %v", i, b1[i], b2[i])
		}
	}
}

func BenchmarkDumpRequest(b *testing.B) {
	benchmarks := []struct {
		name string
		f    func(*http.Request) ([]byte, error)
		body string
	}{
		{"Custom", DumpRequest, "{'hello': 'world!'}"},
		{"Standard", func(req *http.Request) ([]byte, error) {
			return httputil.DumpRequest(req, true)
		}, "{'hello': 'world!'}"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			req, err := http.NewRequest("POST", "http://example.com", strings.NewReader(bm.body))
			if err != nil {
				b.Fatal(err)
			}

			for i := 0; i < b.N; i++ {
				if _, err := bm.f(req); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestDumpResponse(t *testing.T) {
	resp := &http.Response{
		Status: "200",
		Body:   ioutil.NopCloser(strings.NewReader("{'hello': 'world!'}")),
	}

	b1, err := httputil.DumpResponse(resp, true)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := DumpResponse(resp)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(b1))
	t.Log(string(b2))

	for i := range b1 {
		if b1[i] != b2[i] {
			t.Fatalf("Mismatch at %d: %v %v", i, b1[i], b2[i])
		}
	}
}

func BenchmarkDumpResponse(b *testing.B) {
	benchmarks := []struct {
		name string
		f    func(*http.Response) ([]byte, error)
		body string
	}{
		{"Custom", DumpResponse, "{'hello': 'world!'}"},
		{"Standard", func(resp *http.Response) ([]byte, error) {
			return httputil.DumpResponse(resp, true)
		}, "{'hello': 'world!'}"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			resp := &http.Response{
				Status: "200",
				Body:   ioutil.NopCloser(strings.NewReader(bm.body)),
			}

			for i := 0; i < b.N; i++ {
				if _, err := bm.f(resp); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
