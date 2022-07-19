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
		PluckUsers(ctx context.Context, uuids []string) ([]User, error)
	}
	Client struct {
		client       *http.Client
		ApiUrl       string
		errorHandler ErrorHandler
	}
	ErrorHandler func(writer http.ResponseWriter, request *http.Request, statusCode int, err error)
	User         struct {
		ID           int    `json:"id"`
		UUID         string `json:"uuid"`
		Title        string `json:"title"`
		FirstName    string `json:"first_name"`
		LastName     string `json:"last_name"`
		Email        string `json:"email"`
		Role         int    `json:"role"`
		Token        string `json:"token"`
		CreatedAt    int64  `json:"created_at"`
		UpdatedAt    int64  `json:"updated_at"`
		DeletedAt    int64  `json:"deleted_at"`
		Organization struct {
			ID      int    `db:"id" json:"id"`
			Name    string `db:"name" json:"name"`
			Slug    string `db:"slug" json:"slug"`
			Created int64  `db:"created" json:"created"`
		} `json:"organization"`
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
		client: http.DefaultClient,
		ApiUrl: "https://sso.api.lacunacloud.com/api/v1",
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
