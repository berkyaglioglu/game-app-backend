

package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)


func main() {
	fmt.Println("Starting the application...")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, _ = mongo.Connect(ctx, clientOptions)
	
	router := mux.NewRouter()
	
	router.HandleFunc("/leaderboard", GetLeaderboardEndpoint).Methods("GET")
	router.HandleFunc("/leaderboard/{country_iso_code}", GetLeaderboardEndpoint).Methods("GET")
	
	router.HandleFunc("/score/submit", SubmitScoreEndpoint).Methods("POST")
	
	router.HandleFunc("/user/profile/{guid}", GetUserEndpoint).Methods("GET")
	
	router.HandleFunc("/user/create", CreateUserEndpoint).Methods("POST")

	router.HandleFunc("/users/add", AddUsersEndpoint).Methods("POST")

	http.ListenAndServe(":8080", router)
}