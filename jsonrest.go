package jsonrest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// A Request represents a RESTful HTTP request received by the server.
type Request struct {
	route   string
	meta    sync.Map
	context *gin.Context
}

// BasicAuth returns the username and password, if the request uses HTTP Basic
// Authentication.
func (r *Request) BasicAuth() (username, password string, ok bool) {
	return r.context.Request.BasicAuth()
}

// BindBody unmarshals the request body into the given value.
func (r *Request) BindBody(val interface{}) error {
	err := r.context.ShouldBind(val)
	if err != nil {
		msg := "malformed or unexpected json"
		if details := jsonErrorDetails(err); details != "" {
			msg += ": " + details
		}

		return BadRequest(msg).Wrap(err)
	}

	return nil
}

// FormFile returns the first file for the provided form key.
func (r *Request) FormFile(name string, maxMultipartMemory int64) (multipart.File, *multipart.FileHeader, error) {
	if err := r.context.Request.ParseMultipartForm(maxMultipartMemory); err != nil {
		return nil, nil, BadRequest("cannot parse multipart form").Wrap(err)
	}

	return r.context.Request.FormFile(name)
}

// Get returns the meta value for the key.
func (r *Request) Get(key interface{}) interface{} {
	val, _ := r.meta.Load(key)
	return val
}

// Header retrieves a header value by name.
func (r *Request) Header(name string) string {
	return r.context.Request.Header.Get(name)
}

// Param retrieves a URL parameter value by name.
func (r *Request) Param(name string) string {
	return r.context.Param(name)
}

// Query retrieves a querystring value by name.
func (r *Request) Query(name string) string {
	return r.context.Request.URL.Query().Get(name)
}

// Raw returns the underlying *http.Request.
func (r *Request) Raw() *http.Request {
	return r.context.Request
}

// Route returns the route pattern.
func (r *Request) Route() string {
	return r.route
}

// Method returns the HTTP method.
func (r *Request) Method() string {
	return r.context.Request.Method
}

// SetResponseHeader sets a response header.
func (r *Request) SetResponseHeader(key, val string) {
	r.context.Writer.Header().Set(key, val)
}

// Set sets a meta value for the key.
func (r *Request) Set(key, val interface{}) {
	r.meta.Store(key, val)
}

// URL returns the URI being requested from the server.
func (r *Request) URL() *url.URL {
	return r.context.Request.URL
}

// M is a shorthand for map[string]interface{}. Responses from the server may be
// of this type.
type M map[string]interface{}

// An Endpoint is an implementation of a RESTful endpoint.
type Endpoint func(ctx context.Context, r *Request) (interface{}, error)

// Middleware is a function that wraps an endpoint to add new behaviour.
//
// For example, you might create a logging middleware that looks like:
//
//     func LoggingMiddleware(logger *logger.Logger) Middleware {
//         return func(next jsonrest.Endpoint) jsonrest.Endpoint {
//             return func(ctx context.Context, req *jsonrest.Request) (interface{}, error) {
//                 start := time.Now()
//                 defer func() {
//                     log.Printf("%s (%v)", req.URL.Path, time.Since(start))
//                 }()
//                 return next(ctx, req)
//             }
//         }
//    }
type Middleware func(Endpoint) Endpoint

// A Router is an http.Handler that routes incoming requests to registered
// endpoints.
type Router struct {
	// DumpErrors indicates if internal errors should be displayed in the
	// response; useful for local debugging.
	DumpErrors bool

	// notFound is a configurable http.Handler which is called when no matching
	// route is found. If it is not set, notFoundHandler is used.
	notFound gin.HandlerFunc

	router     *gin.Engine
	middleware []Middleware

	parent *Router
}

type Option func(*Router)

// WithNotFoundHandler is an Option available for NewRouter to configure the
// not found handler.
func WithNotFoundHandler(h gin.HandlerFunc) Option {
	return func(r *Router) {
		r.notFound = h
	}
}

// NewRouter returns a new initialized Router.
func NewRouter(options ...Option) *Router {
	hr := gin.New()
	r := &Router{router: hr}

	for _, option := range options {
		option(r)
	}

	if r.notFound == nil {
		notFoundHandler(r)
	} else {
		hr.NoRoute(r.notFound)
	}

	return r
}

// Use registers a middleware to be used for all routes.
func (r *Router) Use(ms ...Middleware) {
	r.middleware = append(r.middleware, ms...)
}

// Group creates a new subrouter, representing a group of routes, from the given
// Router. This subrouter may have its own middleware, but will also inherit its
// parent's middleware.
func (r *Router) Group() *Router {
	return &Router{
		parent:     r,
		router:     r.router,
		DumpErrors: r.DumpErrors,
	}
}

// RouteMap is a map of a method-path pair to an endpoint. For example:
//
//     jsonrest.RouteMap{
//         "GET  /ping": pingEndpoint,
//         "HEAD /api/check": checkEndpoint,
//         "POST /api/update": updateEndpoint,
//         "PUT  /api/update": updateEndpoint,
//     }
type RouteMap map[string]Endpoint

// Routes registers all routes in the route map. It x panic if an entry is
// malformed.
func (r *Router) Routes(m RouteMap) {
	for p, e := range m {
		parts := strings.Fields(p)
		if len(parts) != 2 {
			panic(fmt.Sprintf("invalid RouteMap: %q", p))
		}
		method, path := parts[0], parts[1]
		r.Handle(method, path, e)
	}
}

// Get is a shortcut for router.Handle(http.MethodGet, path, endpoint).
func (r *Router) Get(path string, endpoint Endpoint) {
	r.Handle(http.MethodGet, path, endpoint)
}

// Head is a shortcut for router.Handle(http.MethodHead, path, endpoint).
func (r *Router) Head(path string, endpoint Endpoint) {
	r.Handle(http.MethodHead, path, endpoint)
}

// Post is a shortcut for router.Handle(http.MethodPost, path, endpoint).
func (r *Router) Post(path string, endpoint Endpoint) {
	r.Handle(http.MethodPost, path, endpoint)
}

// Handle registers a new endpoint to handle the given path and method.
func (r *Router) Handle(method, path string, endpoint Endpoint) {
	endpoint = applyMiddleware(endpoint, r)
	handler := endpointToHandler(endpoint, path, r)
	r.router.Handle(method, path, handler)
}

// ServeHTTP implements the http.Handler interface.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}

// applyMiddleware applies the routers's middleware to the provided endpoint.
func applyMiddleware(e Endpoint, r *Router) Endpoint {
	return func(ctx context.Context, req *Request) (interface{}, error) {
		e, r := e, r // copy into local var in closure.

		// Apply middleware from this router and all parent routers.
		for {
			for i := len(r.middleware) - 1; i >= 0; i-- {
				e = r.middleware[i](e)
			}

			if r.parent == nil {
				break
			}

			r = r.parent
		}

		return e(ctx, req)
	}
}

// endpointToHandler converts an endpoint to an httprouter.Handle function.
func endpointToHandler(e Endpoint, path string, r *Router) func(c *gin.Context) {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic serving %v: %+v", c.Request.RequestURI, r)
				debug.PrintStack()
				sendJSON(c.Writer, 500, unknownError)
			}
		}()

		result, err := e(c.Request.Context(), &Request{
			context: c,
			route:   path,
		})
		if err != nil {
			httpErr := translateError(err, r.DumpErrors)
			sendJSON(c.Writer, httpErr.Status, httpErr)
			return
		}

		sendJSON(c.Writer, 200, result)
	}
}

// sendJSON encodes v as JSON and writes it to the response body. Panics
// if an encoding error occurs.
func sendJSON(w http.ResponseWriter, status int, v interface{}) {
	// TODO: Maybe don't panic? This will encounter an error if the caller
	// closes the response early.
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	if err := enc.Encode(v); err != nil {
		panic(err)
	}
}

// notFoundHandler returns a 404 not found response to the caller.
func notFoundHandler(r *Router) {
	r.router.NoRoute(func(c *gin.Context) {
		c.JSON(404, Error(404, "not_found", "url not found"))
	})
}
