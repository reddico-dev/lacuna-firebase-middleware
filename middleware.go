package fbmiddleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

func abort(w http.ResponseWriter, code int, data map[string]interface{}) {
	buf, err := json.Marshal(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(code)
	w.Write(buf)
}

type data map[string]interface{}

func setContext(r *http.Request, key string, value interface{}) {
	ctx := context.WithValue(r.Context(), key, value)
	*r = *r.WithContext(ctx)
}

//func getContext(r *http.Request, key string, value interface{}) {
//	ctx := context.(r.Context(), key, value)
//	*r = *r.WithContext(ctx)
//}

func Test(admin bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// This middleware is respons

			err := errors.New("test")
			if err != nil {
				abort(w, http.StatusUnauthorized, data{"message": "Test err"})
				return
			}

			// Setter
			setContext(r, "this_is_my_user", "my_user")

			// Getter
			user := r.Context().Value("this_is_my_user")
			fmt.Println(user)

			next.ServeHTTP(w, r)
		})
	}
}
