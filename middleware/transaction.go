package middleware

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/volatiletech/sqlboiler/v4/boil"
)

type ContextKey struct {
	name string
}

var TxCtxKey = &ContextKey{name: "tx"}

func contains(s []string, path string) bool {
	for _, a := range s {
		if a == path {
			return true
		}
	}
	return false
}

type responsewriter struct {
	w    http.ResponseWriter
	buf  bytes.Buffer
	code int
}

func (rw *responsewriter) Header() http.Header {
	return rw.w.Header()
}

func (rw *responsewriter) WriteHeader(statusCode int) {
	rw.code = statusCode
}

func (rw *responsewriter) Write(data []byte) (int, error) {
	return rw.buf.Write(data)
}

func (rw *responsewriter) Done() (int64, error) {
	if rw.code > 0 {
		rw.w.WriteHeader(rw.code)
	}
	return io.Copy(rw.w, &rw.buf)
}

func TransactionHandler(db *sql.DB, paths []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			boil.SetDB(db)

			// Stop introspection queries
			var b map[string]interface{}
			body, _ := ioutil.ReadAll(r.Body)
			r.Body.Close()
			r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
			json.Unmarshal(body, &b)

			// URI do not require transaction, skip
			if !contains(paths, r.RequestURI) || b["operationName"] == "IntrospectionQuery" {
				next.ServeHTTP(w, r)
			} else {
				ctx := r.Context()
				tx, _ := boil.BeginTx(ctx, nil)
				ctx = context.WithValue(ctx, TxCtxKey, tx)
				r = r.WithContext(ctx)
				mw := &responsewriter{w: w}

				next.ServeHTTP(mw, r)

				var body map[string]interface{}
				json.Unmarshal(mw.buf.Bytes(), &body)

				// If response status code >= 400 or body contains errors (graphql response)
				// then rollback transaction, otherwise commit
				if mw.code >= 400 || body["errors"] != nil {
					log.Println("rolling back transaction")
					tx.Rollback()

					if strings.Contains(fmt.Sprintf("%v", body["errors"]), "you are not authorized") {
						http.Error(w, "Malformed Content-Type header", http.StatusUnauthorized)
					}
				} else {
					log.Println("commiting transaction")
					tx.Commit()
				}

				if _, err := mw.Done(); err != nil {
					panic(err)
				}
			}
		})
	}
}

func GetTx(ctx context.Context, tx bool) (contextExecutor boil.ContextExecutor) {
	if tx {
		return boil.ContextExecutor(ctx.Value(TxCtxKey).(*sql.Tx))
	} else {
		return boil.ContextExecutor(ctx.Value("db").(*sql.DB))
	}
}
