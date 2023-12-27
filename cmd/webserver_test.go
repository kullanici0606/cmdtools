package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"golang.org/x/net/html"
)

func TestServeHttpFile(t *testing.T) {
	dir := "/home/user/web"

	appFs := afero.NewMemMapFs()
	appFs.MkdirAll(dir, 0755)

	fileContents := []byte("test file")

	afero.WriteFile(appFs, dir+"/test.txt", fileContents, 0644)

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8000/test.txt", nil)
	resp := httptest.NewRecorder()

	d := newDirectoryServerWithFs(dir, appFs)
	d.ServeHTTP(resp, req)

	got := resp.Result().StatusCode
	want := http.StatusOK
	if got != want {
		t.Errorf("want %v got %v", want, got)
	}

	gotHeader := resp.Result().Header.Get("Content-Type")
	wantHeader := "application/octet-stream"
	if gotHeader != wantHeader {
		t.Errorf("want %v got %v", wantHeader, gotHeader)
	}

	gotBody, err := io.ReadAll(resp.Result().Body)

	if err != nil {
		t.Errorf("Error unwanted: %s", err)
	}

	if !reflect.DeepEqual(gotBody, fileContents) {
		t.Errorf("want %v got %v", fileContents, gotBody)
	}
}

func TestServeHttpFolder(t *testing.T) {
	cases := []struct {
		name      string
		inputUrl  string
		wantTitle string
		wantBody  []string
	}{
		{
			"root directory",
			"http://localhost:8000/",
			"Directory listing for /",
			[]string{
				`<li><a href="foo.txt">foo.txt</a></li>`,
				`<li><a href="subfolder/">subfolder/</a></li>`,
				`<li><a href="test.txt">test.txt</a></li>`,
			},
		},
		{
			"sub directory",
			"http://localhost:8000/subfolder/",
			"Directory listing for /",
			[]string{
				`<li><a href="bar.txt">bar.txt</a></li>`,
			},
		},
	}

	dir := "/home/user/web"

	appFs := afero.NewMemMapFs()
	appFs.MkdirAll(dir, 0755)
	afero.WriteFile(appFs, "/home/user/web/test.txt", []byte(""), 0644)
	afero.WriteFile(appFs, "/home/user/web/foo.txt", []byte(""), 0644)
	appFs.MkdirAll("/home/user/web/subfolder", 0755)
	afero.WriteFile(appFs, "/home/user/web/subfolder/bar.txt", []byte(""), 0644)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, c.inputUrl, nil)
			resp := httptest.NewRecorder()

			d := newDirectoryServerWithFs(dir, appFs)
			d.ServeHTTP(resp, req)

			got := resp.Result().StatusCode
			want := http.StatusOK
			if got != want {
				t.Errorf("want %v got %v", want, got)
			}

			gotHeader := resp.Result().Header.Get("Content-Type")
			wantHeader := "text/html; charset=utf-8"
			if gotHeader != wantHeader {
				t.Errorf("want %v got %v", wantHeader, gotHeader)
			}

			doc, err := html.Parse(resp.Result().Body)
			if err != nil {
				t.Errorf("Error unwanted here: %s", err)
			}

			var crawler func(node *html.Node)
			var gotTitle string
			gotBody := make([]string, 0)

			crawler = func(node *html.Node) {
				if node.Type == html.ElementNode && node.Data == "title" {
					gotTitle = node.FirstChild.Data
					return
				}

				if node.Type == html.ElementNode && node.Data == "li" {
					sw := strings.Builder{}
					html.Render(&sw, node)
					gotBody = append(gotBody, sw.String())
					return
				}

				for c := node.FirstChild; c != nil; c = c.NextSibling {
					crawler(c)
				}
			}

			crawler(doc)

			if gotTitle != c.wantTitle {
				t.Errorf("want %v got %v", c.wantTitle, gotTitle)
			}

			if !reflect.DeepEqual(gotBody, c.wantBody) {
				t.Errorf("want %v got %v", c.wantBody, gotBody)
			}
		})
	}
}

func TestServeHttpForbidden(t *testing.T) {
	cases := []struct {
		name           string
		url            string
		expectedStatus int
	}{
		{"deny root escape", "http://localhost:8000/../../../../../../etc/passwd", http.StatusForbidden},
		{"deny same prefix but different root", "http://localhost:8000/../../../../../../home/user/web2", http.StatusForbidden},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {

			req := httptest.NewRequest(http.MethodGet, c.url, nil)
			resp := httptest.NewRecorder()

			d := newDirectoryServer("/home/user/web")
			d.ServeHTTP(resp, req)

			got := resp.Result().StatusCode
			if got != c.expectedStatus {
				t.Errorf("want %v got %v", c.expectedStatus, got)
			}
		})
	}
}
