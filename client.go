package sso

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type (
	Service interface {
		AuthCheck(adminOnly bool) func(http.Handler) http.Handler
		GetTeam(ctx context.Context) ([]User, error)
		GetAll(ctx context.Context) ([]User, error)
		PluckUsers(ctx context.Context, uuids []string) ([]User, error)
	}
	Client struct {
		client       *http.Client
		ApiUrl       string
		errorHandler ErrorHandler
	}
	ErrorHandler func(writer http.ResponseWriter, request *http.Request, statusCode int, err error)
	Organization struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
		Apps []struct {
			ID          int    `json:"id"`
			Title       string `json:"title"`
			Colour      string `json:"colour"`
			Url         string `json:"url"`
			ShortDesc   string `json:"short_desc"`
			Description string `json:"description"`
			Image       string `json:"image"`
			Category    string `json:"category"`
			Key         string `json:"key"`
			LinearId    string `json:"linear_id"`
			Enabled     bool   `json:"enabled"`
		} `json:"apps"`
		Domain     string `json:"domain"`
		OpenInvite bool   `json:"open_invite"`
		Created    int64  `json:"created"`
	}
	User struct {
		ID            int            `json:"id"`
		UUID          string         `json:"uuid"`
		Title         string         `json:"title"`
		FirstName     string         `json:"first_name"`
		LastName      string         `json:"last_name"`
		Email         string         `json:"email"`
		Role          int            `json:"role"`
		Token         string         `json:"token"`
		CreatedAt     int64          `json:"created_at"`
		UpdatedAt     int64          `json:"updated_at"`
		DeletedAt     int64          `json:"deleted_at"`
		Organization  Organization   `json:"organization"`
		Organizations []Organization `json:"organizations"`
	}
	AuthCheckResponse struct {
		Message string `json:"message"`
		User    *User  `json:"user"`
	}
	UserListResponse struct {
		Message string `json:"message"`
		Users   []User `json:"users"`
	}
)

func New(errorHandler ErrorHandler) *Client {
	if errorHandler == nil {
		errorHandler = DefaultErrorHandler
	}
	return &Client{
		client:       http.DefaultClient,
		ApiUrl:       "https://account.api.lacunacloud.com/api/v1",
		errorHandler: errorHandler,
		//ApiUrl: "http://localhost:5001/api/v1",
	}
}

const AuthHeader = "token"
const UserContextKey = "user"
const UserContentUUIDKey = "user_uuid"

func DefaultErrorHandler(writer http.ResponseWriter, request *http.Request, statusCode int, err error) {
	abort(writer, http.StatusInternalServerError, data{"message": err.Error()})
}

func (c *Client) AuthCheck(adminOnly bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			token := r.Header.Get(AuthHeader)
			appKey := r.Header.Get("app")
			req, err := http.NewRequest(http.MethodPost, c.ApiUrl+"/user/sync", nil)
			if err != nil {
				c.errorHandler(w, r, http.StatusInternalServerError, err)
				return
			}
			req.Header.Set(AuthHeader, token)
			req.Header.Set("app", appKey)

			resp, err := c.client.Do(req)
			if err != nil {
				c.errorHandler(w, r, http.StatusInternalServerError, err)
				return
			}
			defer resp.Body.Close()

			ssoResponse := AuthCheckResponse{}
			err = json.NewDecoder(resp.Body).Decode(&ssoResponse)
			if err != nil {
				c.errorHandler(w, r, http.StatusInternalServerError, err)
				return
			}

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				c.errorHandler(w, r, resp.StatusCode, errors.New(ssoResponse.Message))
				return
			}

			ssoResponse.User.Token = token
			setContext(r, UserContextKey, ssoResponse.User)
			setContext(r, UserContentUUIDKey, ssoResponse.User.UUID)
			setContext(r, "app", appKey)

			if adminOnly && ssoResponse.User.Role > 1 {
				c.errorHandler(w, r, resp.StatusCode, errors.New("admin only"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (c *Client) GetTeam(ctx context.Context) ([]User, error) {
	ssoResponse := UserListResponse{}

	ctxUserData := ctx.Value(UserContextKey)
	appKey := ctx.Value("app")

	// Cast firebase context to struct
	fbUserData, getUserOk := ctxUserData.(*User)
	if !getUserOk {
		return ssoResponse.Users, errors.New("could not cast context to user struct")
	}

	req, err := http.NewRequest(http.MethodGet, c.ApiUrl+"/team/list", nil)
	if err != nil {
		return ssoResponse.Users, err
	}
	req.Header.Set(AuthHeader, fbUserData.Token)
	req.Header.Set("app", appKey.(string))

	resp, err := c.client.Do(req)
	if err != nil {
		return ssoResponse.Users, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&ssoResponse)
	if err != nil {
		return ssoResponse.Users, err
	}

	//fmt.Println(ssoResponse)
	return ssoResponse.Users, nil

}

func (c *Client) GetAll(ctx context.Context) ([]User, error) {
	ssoResponse := UserListResponse{}

	ctxUserData := ctx.Value(UserContextKey)
	appKey := ctx.Value("app")

	// Cast firebase context to struct
	fbUserData, getUserOk := ctxUserData.(*User)
	if !getUserOk {
		return ssoResponse.Users, errors.New("could not cast context to user struct")
	}

	req, err := http.NewRequest(http.MethodGet, c.ApiUrl+"/users/list", nil)
	if err != nil {
		return ssoResponse.Users, err
	}
	req.Header.Set(AuthHeader, fbUserData.Token)
	req.Header.Set("app", appKey.(string))

	resp, err := c.client.Do(req)
	if err != nil {
		return ssoResponse.Users, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&ssoResponse)
	if err != nil {
		return ssoResponse.Users, err
	}

	//fmt.Println(ssoResponse)
	return ssoResponse.Users, nil
}

func (c *Client) PluckUsers(ctx context.Context, uuids []string) ([]User, error) {

	ssoResponse := UserListResponse{}
	ctxUserData := ctx.Value(UserContextKey)
	appKey := ctx.Value("app")
	// Cast firebase context to struct
	fbUserData, getUserOk := ctxUserData.(*User)
	if !getUserOk {
		return ssoResponse.Users, errors.New("could not cast context to user struct")
	}

	body := data{
		"user_ids": strings.Join(uuids, ","),
	}

	bodyBuffer := &bytes.Buffer{}
	err := json.NewEncoder(bodyBuffer).Encode(body)
	if err != nil {
		return ssoResponse.Users, err
	}

	req, err := http.NewRequest(http.MethodGet, c.ApiUrl+"/team/pluck", bodyBuffer)
	if err != nil {
		return ssoResponse.Users, err
	}
	req.Header.Set(AuthHeader, fbUserData.Token)
	req.Header.Set("app", appKey.(string))

	resp, err := c.client.Do(req)
	if err != nil {
		return ssoResponse.Users, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&ssoResponse)
	if err != nil {
		return ssoResponse.Users, err
	}

	//fmt.Println(ssoResponse.Users)
	return ssoResponse.Users, nil

}

type ApiEvent struct {
	ID            int    `json:"id" id:"id"`
	UserID        int    `json:"user_id" id:"user_id"`
	App           string `json:"app" id:"app"`
	Endpoint      string `json:"endpoint" id:"endpoint"`
	Method        string `json:"method" id:"method"`
	Time          int64  `json:"time" id:"time"`
	Address       string `json:"address" id:"address"`
	WorkspaceSlug string `json:"workspace_slug" id:"workspace_slug"`
}

func (c *Client) Usage() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			go func() {
				req, err := http.NewRequest(http.MethodPost, c.ApiUrl+"/activity/log", nil)
				if err != nil {
					c.errorHandler(w, r, http.StatusInternalServerError, err)
					return
				}

				req.Header.Set("token", r.Header.Get("token"))
				req.Header.Set("app", r.Header.Get("app"))
				req.Header.Set("endpoint", r.URL.Path)
				req.Header.Set("method", r.Method)
				req.Header.Set("address", r.RemoteAddr)

				resp, err := c.client.Do(req)
				if err != nil {
					c.errorHandler(w, r, resp.StatusCode, err)
					return
				}

				if resp.StatusCode != 200 {
					c.errorHandler(w, r, resp.StatusCode, err)
					return
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
