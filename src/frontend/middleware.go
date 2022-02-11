package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ctxKeyLog struct{}
type ctxKeyRequestID struct{}

type logHandler struct {
	log  *logrus.Logger
	next http.Handler
}

type responseRecorder struct {
	b      int
	status int
	w      http.ResponseWriter
}

func (r *responseRecorder) Header() http.Header { return r.w.Header() }

func (r *responseRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.w.Write(p)
	r.b += n
	return n, err
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.w.WriteHeader(statusCode)
}

func (lh *logHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID, _ := uuid.NewRandom()
	ctx = context.WithValue(ctx, ctxKeyRequestID{}, requestID.String())
	start := time.Now()
	rr := &responseRecorder{w: w}
	log := lh.log.WithFields(logrus.Fields{
		"http.req.path":   r.URL.Path,
		"http.req.method": r.Method,
		"http.req.id":     requestID.String(),
	})

	if v, ok := r.Context().Value(ctxKeySessionID{}).(string); ok {
		log = log.WithField("session", v)
	}
	log.Debug("request started")
	defer func() {
		log.WithFields(logrus.Fields{
			"http.resp.took_ms": int64(time.Since(start) / time.Millisecond),
			"http.resp.status":  rr.status,
			"http.resp.bytes":   rr.b}).Debugf("request complete")
	}()

	ctx = context.WithValue(ctx, ctxKeyLog{}, log)
	r = r.WithContext(ctx)
	lh.next.ServeHTTP(rr, r)
}

func ensureSessionID(next http.Handler) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		var sessionID string
		userAgent := r.UserAgent()
		rnd := rand.Intn(100) + 1

		// DEMO: If the checkoutservice Cache size is greater than the userThreshold (default 35000)
		// AND the request is from the load generator (useragent contains python)
		// AND rnd > PercentNormal
		// Then we will use a session id of 20109 to emphasize a problematic user
		// sessionID will be referenced as userid in OpenTelemetry data
		if CacheTrack.IsOverUserThreshold() && strings.Contains(userAgent, "python") && rnd > PercentNormal {
			// Use the static sessionID "20109"
			sessionID = "20109"
			http.SetCookie(w, &http.Cookie{
				Name:   cookieSessionID,
				Value:  sessionID,
				MaxAge: cookieMaxAge,
			})
		} else {
			// generate a sparse but random-looking set of session IDs
			rsession := rand.Intn(25) + (rand.Intn(25) * 100) + (rand.Intn(25) * 10000)
			sessionID = strconv.Itoa(rsession)

			c, err := r.Cookie(cookieSessionID)
			if err == http.ErrNoCookie {
				http.SetCookie(w, &http.Cookie{
					Name:   cookieSessionID,
					Value:  sessionID,
					MaxAge: cookieMaxAge,
				})
			} else if err != nil {
				return
			} else {
				sessionID = c.Value
			}
		}

		ctx := context.WithValue(r.Context(), ctxKeySessionID{}, sessionID)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	}
}
