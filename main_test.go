
package main

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "context"
    "time"
    "bytes"
    "encoding/json"
    "io/ioutil"

    "github.com/gorilla/mux"
    "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
    "github.com/stretchr/testify/assert"
)

func Router() *mux.Router {
    router := mux.NewRouter()

    router.HandleFunc("/leaderboard", GetLeaderboardEndpoint).Methods("GET")
    router.HandleFunc("/leaderboard/{country_iso_code}", GetLeaderboardEndpoint).Methods("GET")
	
	router.HandleFunc("/score/submit", SubmitScoreEndpoint).Methods("POST")
	
	router.HandleFunc("/user/profile/{guid}", GetUserEndpoint).Methods("GET")
	
	router.HandleFunc("/user/create", CreateUserEndpoint).Methods("POST")

	router.HandleFunc("/users/add", AddUsersEndpoint).Methods("POST")

    return router
}

func init() {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, _ = mongo.Connect(ctx, clientOptions)

	type  UserNumber struct {
		Number   int `json:"number_of_users"`
		MaxPoint int `json:"max_point"`
	}
	userNumber := &UserNumber{
        Number: 10,
        MaxPoint: 100,
    }
    jsonUserNumber, _ := json.Marshal(userNumber)
    request, _ := http.NewRequest("POST", "/users/add", bytes.NewBuffer(jsonUserNumber))
    response := httptest.NewRecorder()

    Router().ServeHTTP(response, request)
}



func TestGetLeaderboardEndpoint(t *testing.T) {
    request, _ := http.NewRequest("GET", "/leaderboard", nil)
    response := httptest.NewRecorder()

    Router().ServeHTTP(response, request)
    assert.Equal(t, 200, response.Code, "HTTP 200 response code is expected")

    responseData, _ := ioutil.ReadAll(response.Body)
    responseString := string(responseData)
    users := &[]User{ }
    _ = json.Unmarshal([]byte(responseString), users)
    
    for i, user := range (*users) {
    	rank := i + 1
    	assert.Equal(t, rank, user.Rank, "Rank is expected in order")
    }
}

func TestSubmitScoreEndpoint(t *testing.T) {
	// get users
	request, _ := http.NewRequest("GET", "/leaderboard", nil)
    response := httptest.NewRecorder()

    Router().ServeHTTP(response, request)
    assert.Equal(t, 200, response.Code, "HTTP 200 response code is expected")

    responseData, _ := ioutil.ReadAll(response.Body)
    responseString := string(responseData)
    users := &[]User{ }
    _ = json.Unmarshal([]byte(responseString), users)

    // create score
    type Score struct {
		ScoreWorth float64 `json:"score_worth"`
		UserID     string  `json:"user_id"`
	}
	userID := (*users)[len(*users)-1].ID // last user's id
	score := &Score{
		ScoreWorth: 10,
		UserID: userID,
	}
    
    prevPoints := (*users)[len(*users)-1].Points

    jsonScore, _ := json.Marshal(score)
    request, _ = http.NewRequest("POST", "/score/submit", bytes.NewBuffer(jsonScore))
    response = httptest.NewRecorder()

    Router().ServeHTTP(response, request)
    assert.Equal(t, 200, response.Code, "HTTP 200 response code is expected")

    // check ranks are still in order
    request, _ = http.NewRequest("GET", "/leaderboard", nil)
    response = httptest.NewRecorder()

    Router().ServeHTTP(response, request)
    assert.Equal(t, 200, response.Code, "HTTP 200 response code is expected")

    responseData, _ = ioutil.ReadAll(response.Body)
    responseString = string(responseData)
    users = &[]User{ }
    _ = json.Unmarshal([]byte(responseString), users)
    
    for i, user := range (*users) {
    	if user.ID == userID {
    		assert.Equal(t, user.Points, prevPoints + 10, "User's points should change")
    	}
    	rank := i + 1
    	assert.Equal(t, rank, user.Rank, "Rank is expected in order after score submission")
    }
}

func TestGetUserEndpoint(t *testing.T) {
	// get users
	request, _ := http.NewRequest("GET", "/leaderboard", nil)
    response := httptest.NewRecorder()

    Router().ServeHTTP(response, request)
    assert.Equal(t, 200, response.Code, "HTTP 200 response code is expected")

    responseData, _ := ioutil.ReadAll(response.Body)
    responseString := string(responseData)
    users := &[]User{ }
    _ = json.Unmarshal([]byte(responseString), users)


    userID := (*users)[0].ID
	// get specific user's profile
    request, _ = http.NewRequest("GET", "/user/profile/"+userID, nil)
    response = httptest.NewRecorder()

    Router().ServeHTTP(response, request)
    assert.Equal(t, 200, response.Code, "HTTP 200 response code is expected")
}

func TestCreateUserEndpoint(t *testing.T) {
	// create user
	user := &User{
        Points: 10,
        DisplayName: "gjg",
        Country: "tr",
    }
    jsonUser, _ := json.Marshal(user)
    request, _ := http.NewRequest("POST", "/user/create", bytes.NewBuffer(jsonUser))
    response := httptest.NewRecorder()

    Router().ServeHTTP(response, request)
    assert.Equal(t, 201, response.Code, "HTTP 200 response code is expected")

    // check ranks are still in order
    request, _ = http.NewRequest("GET", "/leaderboard", nil)
    response = httptest.NewRecorder()

    Router().ServeHTTP(response, request)
    assert.Equal(t, 200, response.Code, "HTTP 200 response code is expected")

    responseData, _ := ioutil.ReadAll(response.Body)
    responseString := string(responseData)
    users := &[]User{ }
    _ = json.Unmarshal([]byte(responseString), users)
    
    for i, user := range (*users) {
    	rank := i + 1
    	assert.Equal(t, rank, user.Rank, "Rank is expected in order after creating user")
    }
}