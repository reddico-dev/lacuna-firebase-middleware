package main

import (
	"encoding/json"
	"fmt"
	fbmiddleware "github.com/reddico-dev/lacuna-firebase-middleware"
	"log"
	"net/http"
)

func main() {
	fmt.Println("hello")

	client, err := fbmiddleware.New(nil)
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/auth", client.AuthCheck(true)(HandlePing()))
	http.Handle("/team", client.AuthCheck(true)(HandleGetTeam(client)))
	http.Handle("/pluck", client.AuthCheck(true)(HandlePluckUsers(client)))
	log.Fatal(http.ListenAndServe(":6767", nil))
}

func HandlePing() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

func HandleGetTeam(client *fbmiddleware.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		team, err := client.GetTeam(r.Context())
		fmt.Println(team, err)
		Respond(w, team, err)
	}
}

func HandlePluckUsers(client *fbmiddleware.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := client.PluckUsers(r.Context(), []string{"development"})
		Respond(w, users, err)
	}
}

func Respond(w http.ResponseWriter, data interface{}, err error) {
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	buf, err := json.Marshal(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(buf)
}
