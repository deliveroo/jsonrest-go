package jsonrest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/deliveroo/assert-go"
	"github.com/deliveroo/jsonrest-go"
)

func TestSimpleGet(t *testing.T) {
	r := jsonrest.NewRouter()
	r.Get("/hello", func(ctx context.Context, r *jsonrest.Request) (interface{}, error) {
		return jsonrest.M{"message": "Hello World"}, nil
	})

	w := do(r, http.MethodGet, "/hello", nil, "application/json")
	assert.Equal(t, w.Result().StatusCode, 200)
	assert.JSONEqual(t, w.Body.String(), m{"message": "Hello World"})
}

func TestRequestBody(t *testing.T) {
	r := jsonrest.NewRouter()
	r.Post("/users", func(ctx context.Context, r *jsonrest.Request) (interface{}, error) {
		var params struct {
			ID int `json:"id"`
		}
		if err := r.BindBody(&params); err != nil {
			return nil, err
		}
		return jsonrest.M{"id": params.ID}, nil
	})

	t.Run("good json", func(t *testing.T) {
		w := do(r, http.MethodPost, "/users", strings.NewReader(`{"id": 1}`), "application/json")
		assert.Equal(t, w.Result().StatusCode, 200)
		assert.JSONEqual(t, w.Body.String(), m{"id": 1})
	})

	t.Run("bad json", func(t *testing.T) {
		w := do(r, http.MethodPost, "/users", strings.NewReader(`{"id": |1}`), "application/json")
		assert.Equal(t, w.Result().StatusCode, 400)
		assert.JSONEqual(t, w.Body.String(), m{
			"error": m{
				"code":    "bad_request",
				"message": "malformed or unexpected json: offset 8: invalid character '|' looking for beginning of value",
			},
		})
	})
}

func TestFormFile(t *testing.T) {
	const defaultMaxMemory = 32 << 20
	r := jsonrest.NewRouter()
	r.Post("/file_upload", func(ctx context.Context, r *jsonrest.Request) (interface{}, error) {
		f, fh, err := r.FormFile("file", defaultMaxMemory)
		if err != nil {
			return nil, err
		}
		f.Close()
		return jsonrest.M{"fileName": fh.Filename}, nil
	})

	t.Run("good file", func(t *testing.T) {
		buf := new(bytes.Buffer)
		mw := multipart.NewWriter(buf)
		w, err := mw.CreateFormFile("file", "test")
		assert.Must(t, err)
		_, err = w.Write([]byte("test"))
		assert.Must(t, err)
		mw.Close()

		r := do(r, http.MethodPost, "/file_upload", buf, mw.FormDataContentType())
		assert.Equal(t, r.Result().StatusCode, 200)
		assert.JSONEqual(t, r.Body.String(), m{"fileName": "test"})
	})

	t.Run("an empty file", func(t *testing.T) {
		buf := new(bytes.Buffer)
		mw := multipart.NewWriter(buf)

		r := do(r, http.MethodPost, "/file_upload", buf, mw.FormDataContentType())
		assert.Equal(t, r.Result().StatusCode, 400)
	})
}

func TestRequestURLParams(t *testing.T) {
	r := jsonrest.NewRouter()
	r.Get("/users/:id", func(ctx context.Context, r *jsonrest.Request) (interface{}, error) {
		id := r.Param("id")
		if id == "" {
			return nil, errors.New("missing id")
		}
		return jsonrest.M{"id": id}, nil
	})

	w := do(r, http.MethodGet, "/users/123", nil, "application/json")
	assert.Equal(t, w.Result().StatusCode, 200)
	assert.JSONEqual(t, w.Body.String(), m{"id": "123"})
}

func TestNotFound(t *testing.T) {
	t.Run("no override", func(t *testing.T) {
		r := jsonrest.NewRouter()
		w := do(r, http.MethodGet, "/invalid_path", nil, "application/json")
		assert.Equal(t, w.Result().StatusCode, 404)
		assert.JSONEqual(t, w.Body.String(), m{
			"error": m{
				"code":    "not_found",
				"message": "url not found",
			},
		})
	})

	t.Run("with override", func(t *testing.T) {
		h := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("content-type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			assert.Must(t, json.NewEncoder(w).Encode(m{"proxy": true}))
		})
		r := jsonrest.NewRouter(jsonrest.WithNotFoundHandler(h))
		w := do(r, http.MethodGet, "/invalid_path", nil, "application/json")
		assert.Equal(t, w.Result().StatusCode, 200)
		assert.JSONEqual(t, w.Body.String(), m{
			"proxy": true,
		})
	})
}

func TestError(t *testing.T) {
	tests := []struct {
		err        error
		wantStatus int
		want       interface{}
	}{
		{
			errors.New("missing id"),
			500, m{
				"error": m{
					"code":    "unknown_error",
					"message": "an unknown error occurred",
				},
			},
		},
		{
			jsonrest.Error(404, "customer_not_found", "customer not found"),
			404, m{
				"error": m{
					"code":    "customer_not_found",
					"message": "customer not found",
				},
			},
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := jsonrest.NewRouter()
			r.Get("/fail", func(ctx context.Context, r *jsonrest.Request) (interface{}, error) {
				return nil, tt.err
			})

			w := do(r, http.MethodGet, "/fail", nil, "application/json")
			assert.Equal(t, w.Result().StatusCode, tt.wantStatus)
			assert.JSONEqual(t, w.Body.String(), tt.want)
		})
	}
}

func TestDumpInternalError(t *testing.T) {
	r := jsonrest.NewRouter()
	r.DumpErrors = true
	r.Get("/", func(ctx context.Context, r *jsonrest.Request) (interface{}, error) {
		return nil, errors.New("foo error occurred")
	})

	w := do(r, http.MethodGet, "/", nil, "application/json")
	assert.Equal(t, w.Result().StatusCode, 500)
	assert.JSONEqual(t, w.Body.String(), m{
		"error": m{
			"code":    "unknown_error",
			"message": "an unknown error occurred",
			"details": []string{
				"foo error occurred",
			},
		},
	})
}

func TestMiddleware(t *testing.T) {
	t.Run("top level middleware", func(t *testing.T) {
		r := jsonrest.NewRouter()
		called := false
		r.Use(func(next jsonrest.Endpoint) jsonrest.Endpoint {
			return func(ctx context.Context, req *jsonrest.Request) (interface{}, error) {
				called = true
				return next(ctx, req)
			}
		})
		r.Get("/test", func(ctx context.Context, req *jsonrest.Request) (interface{}, error) { return nil, nil })

		w := do(r, http.MethodGet, "/test", nil, "application/json")
		assert.Equal(t, w.Result().StatusCode, 200)
		assert.True(t, called)
	})
	t.Run("group", func(t *testing.T) {
		r := jsonrest.NewRouter()
		called := false

		withMiddleware := r.Group()
		withMiddleware.Use(func(next jsonrest.Endpoint) jsonrest.Endpoint {
			return func(ctx context.Context, req *jsonrest.Request) (interface{}, error) {
				called = true
				return next(ctx, req)
			}
		})
		withMiddleware.Get("/withmiddleware", func(ctx context.Context, req *jsonrest.Request) (interface{}, error) { return nil, nil })

		withoutMiddleware := r.Group()
		withoutMiddleware.Get("/withoutmiddleware", func(ctx context.Context, req *jsonrest.Request) (interface{}, error) { return nil, nil })

		w := do(r, http.MethodGet, "/withmiddleware", nil, "application/json")
		assert.Equal(t, w.Result().StatusCode, 200)
		assert.True(t, called)

		called = false
		w = do(r, http.MethodGet, "/withoutmiddleware", nil, "application/json")
		assert.Equal(t, w.Result().StatusCode, 200)
		assert.False(t, called)
	})
}

type m map[string]interface{}

func do(h http.Handler, method, path string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}
