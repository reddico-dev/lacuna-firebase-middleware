package fbmiddleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/lacuna-seo/stash"
	"net/http"
	"strings"
)

type (
	Client struct {
		client *http.Client
		cache  stash.Store
		ApiUrl string
	}
	User struct {
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

func New(cache stash.Store) (*Client, error) {
	return &Client{
		client: http.DefaultClient,
		cache:  cache,
		ApiUrl: "https://sso.api.lacunacloud.com/api/v1",
		//ApiUrl: "http://localhost:5001/api/v1",
	}, nil
}

const AuthHeader = "token"
const UserContextKey = "user"
const UserContentUUIDKey = "user_uuid"

func (c *Client) AuthCheck(adminOnly bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			token := r.Header.Get(AuthHeader)
			appKey := r.Header.Get("app")
			req, err := http.NewRequest(http.MethodPost, c.ApiUrl+"/user/sync", nil)
			if err != nil {
				abort(w, http.StatusInternalServerError, data{"message": err.Error()})
				return
			}
			req.Header.Set(AuthHeader, token)
			req.Header.Set("app", appKey)

			resp, err := c.client.Do(req)
			if err != nil {
				abort(w, http.StatusInternalServerError, data{"message": err.Error()})
				return
			}
			defer resp.Body.Close()

			ssoResponse := AuthCheckResponse{}
			err = json.NewDecoder(resp.Body).Decode(&ssoResponse)
			if err != nil {
				abort(w, http.StatusInternalServerError, data{"message": err.Error()})
				return
			}

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				abort(w, resp.StatusCode, data{"message": ssoResponse.Message})
				return
			}

			ssoResponse.User.Token = token
			setContext(r, UserContextKey, ssoResponse.User)
			setContext(r, UserContentUUIDKey, ssoResponse.User.UUID)
			setContext(r, "app", appKey)

			if adminOnly && ssoResponse.User.Role > 1 {
				abort(w, http.StatusUnauthorized, data{"message": "Admin only"})
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
