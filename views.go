
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"math/rand"
	"time"
	"sort"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)




var client *mongo.Client






// HELPER FUNCTIONS START

func GetRankReplacer(sliceID int) RankReplacer {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	collectionRankReplacer := client.Database("GoodJobGames").Collection("RankReplacer")
	
	var rankReplacer RankReplacer
	_ = collectionRankReplacer.FindOne(ctx, bson.M{"slice_id": sliceID}).Decode(&rankReplacer)
	
	return rankReplacer
}


// function takes (user, user's current actual rank)
func FindRankAndUpdate(user User, oldRankActual int) {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	collectionUser := client.Database("GoodJobGames").Collection("User")
	collectionRankReplacer := client.Database("GoodJobGames").Collection("RankReplacer")

	sliceLength := 100000

	var rankReplacerSlices []RankReplacer
	
	// find all the documents where slice id less than or equal to slice id of the document that contains old ranking information
	// because new rank will be either in the current document or another document with smaller slice id
	cursor, _ := collectionRankReplacer.Find(ctx, bson.M{ })
	
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var rankReplacer RankReplacer
		cursor.Decode(&rankReplacer)
		rankReplacerSlices = append(rankReplacerSlices, rankReplacer)
	}
	// sort rankReplacerSlices list according to slice_id
	sort.Slice(rankReplacerSlices, func(i, j int) bool {
		return rankReplacerSlices[i].SliceID < rankReplacerSlices[j].SliceID
	})

	var tempUser User

	// just helpful variables in the  following steps
	var rankUser int
	var sliceID int

	// mid, left, right for binary search
	var midRankActual int
	leftRankActual := 1
	rightRankActual := oldRankActual - 1

	// binary search to find the new rank of the user
	for leftRankActual < rightRankActual {
		midRankActual = (leftRankActual + rightRankActual) / 2

		// find the rank index in particular slice in which midRankActual is corresponding to
		sliceID = (midRankActual - 1) / sliceLength + 1
		if midRankActual % sliceLength == 0 {
			rankUser = rankReplacerSlices[sliceID-1].ActualToUser[sliceLength]
		} else {
			rankUser = rankReplacerSlices[sliceID-1].ActualToUser[midRankActual % sliceLength]
		}
		

		_ = collectionUser.FindOne(ctx, bson.M{"rank": rankUser}).Decode(&tempUser)
		
		if (user.Points > tempUser.Points) || (user.Points == tempUser.Points && user.DisplayName < tempUser.DisplayName) {
			rightRankActual = midRankActual - 1
		} else {
			leftRankActual = midRankActual + 1
		}
	}

	var newRankActual int

	if rightRankActual < leftRankActual {
		newRankActual = leftRankActual
	} else { // only option left == right
		// find the rank index in particular slice in which leftRankActual is corresponding to
		sliceID = (leftRankActual - 1) / sliceLength + 1
		if leftRankActual % sliceLength == 0 {
			rankUser = rankReplacerSlices[sliceID-1].ActualToUser[sliceLength]
		} else {
			rankUser = rankReplacerSlices[sliceID-1].ActualToUser[leftRankActual % sliceLength]
		}

		_ = collectionUser.FindOne(ctx, bson.M{"rank": rankUser}).Decode(&tempUser)
		// when the points are equal, lower display name has better rank
		if (user.Points > tempUser.Points) || (user.Points == tempUser.Points && user.DisplayName < tempUser.DisplayName) {
			newRankActual = leftRankActual
		} else {
			newRankActual = leftRankActual + 1
		}
	}


	rankUser = user.Rank
	var tempRankUser int
	slicesToBeUpdated := map[int]bool{}
	// modify the ranks which should change
	for rankActual := newRankActual; rankActual <= oldRankActual; rankActual++ {
		sliceID = (rankActual - 1) / sliceLength + 1
		if rankActual % sliceLength == 0 {
			tempRankUser = rankReplacerSlices[sliceID-1].ActualToUser[sliceLength]
			rankReplacerSlices[sliceID-1].ActualToUser[sliceLength] = rankUser
		} else {
			tempRankUser = rankReplacerSlices[sliceID-1].ActualToUser[rankActual % sliceLength]
			rankReplacerSlices[sliceID-1].ActualToUser[rankActual % sliceLength] = rankUser
		}
		slicesToBeUpdated[sliceID] = true
		
		sliceID = (rankUser - 1) / sliceLength + 1
		if rankUser % sliceLength == 0 {
			rankReplacerSlices[sliceID-1].UserToActual[sliceLength] = rankActual
		} else {
			rankReplacerSlices[sliceID-1].UserToActual[rankUser % sliceLength] = rankActual
		}
		slicesToBeUpdated[sliceID] = true
		
		rankUser = tempRankUser
	}
	
	// update the modified slices in db
	for sliceID, _ := range slicesToBeUpdated {
		// update RankReplacer document
		_, _ = collectionRankReplacer.UpdateOne(
			ctx,
			bson.M{"slice_id": sliceID},
			bson.M{"$set": bson.M{"user_to_actual": rankReplacerSlices[sliceID-1].UserToActual, "actual_to_user": rankReplacerSlices[sliceID-1].ActualToUser}},
		)
	}

}

// HELPER FUNCTIONS END






//------------------------ ENDPOINTS START HERE ------------------------


func GetLeaderboardEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("content-type", "application/json")

	sliceLength := 100000
	
	var users []User

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	collectionUser := client.Database("GoodJobGames").Collection("User")
	collectionRankReplacer := client.Database("GoodJobGames").Collection("RankReplacer")

	count, _ := collectionUser.CountDocuments(ctx, bson.M{})
	if count == 0 {
		return
	}

	params := mux.Vars(request)
	var cursor *mongo.Cursor
	var err error
	if code, ok := params["country_iso_code"]; ok {
		cursor, err = collectionUser.Find(ctx, bson.D{{"country", code}})
	} else {
		cursor, err = collectionUser.Find(ctx, bson.D{})
	}

	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
		return
	}

	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var user User
		cursor.Decode(&user)
		users = append(users, user)
	}
	if err = cursor.Err(); err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
		return
	}

	// Get all rank replacer slices to set users' actual ranks when displaying
	var rankReplacerSlices []RankReplacer
	cursor, _ = collectionRankReplacer.Find(ctx, bson.M{ })
	
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var rankReplacer RankReplacer
		cursor.Decode(&rankReplacer)
		rankReplacerSlices = append(rankReplacerSlices, rankReplacer)
	}
	// sort rankReplacerSlices list according to slice_id
	sort.Slice(rankReplacerSlices, func(i, j int) bool {
		return rankReplacerSlices[i].SliceID < rankReplacerSlices[j].SliceID
	})

	// now set users' actual ranks
	for i := 0; i < len(users); i++ {
		// e.g. here if user rank = 100,000 , can find actual rank in document whose slice id = 1. If it was 100,001 , then slice id = 2.
		sliceID := (users[i].Rank - 1) / sliceLength + 1
		if users[i].Rank % sliceLength == 0 {
			users[i].Rank = rankReplacerSlices[sliceID-1].UserToActual[sliceLength]
		} else {
			users[i].Rank = rankReplacerSlices[sliceID-1].UserToActual[users[i].Rank % sliceLength]
		}
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].Rank < users[j].Rank
	})

	response.WriteHeader(200)
	json.NewEncoder(response).Encode(users)
}


func SubmitScoreEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("content-type", "application/json")

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	collectionUser := client.Database("GoodJobGames").Collection("User")
	
	sliceLength := 100000
	
	type Score struct {
		ScoreWorth float64 `json:"score_worth"`
		UserID     string  `json:"user_id"`
		Timestamp  int64   `json:"timestamp"`
	}
	var score Score
	_ = json.NewDecoder(request.Body).Decode(&score)
	score.Timestamp = time.Now().Unix()

	if score.ScoreWorth == 0 {
		json.NewEncoder(response).Encode(score)
		return
	}

	// find user whose rank could be changed
	var user User
	err := collectionUser.FindOne(ctx, bson.M{"_id": score.UserID}).Decode(&user)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
		return
	}

	//increment user's point
	user.Points += score.ScoreWorth

	// e.g. here if user rank = 100,000 , can find actual rank in document whose slice id = 1. If it was 100,001 , then slice id = 2.
	sliceID := (user.Rank - 1) / sliceLength + 1
	// find the new rank of user and update necessary ranks on RankReplacement document
	rankReplacer := GetRankReplacer(sliceID)
	var rankUser int
	if user.Rank % sliceLength == 0 {
		rankUser = sliceLength
	} else {
		rankUser = user.Rank % sliceLength
	}
	rankActual := rankReplacer.UserToActual[rankUser]
	FindRankAndUpdate(user, rankActual)

	// update the user's point
	_, err = collectionUser.UpdateOne(
		ctx,
		bson.M{"_id": user.ID},
		bson.M{"$set": bson.M{"points": user.Points}},
	)

	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
		return
	}
	response.WriteHeader(200)
	json.NewEncoder(response).Encode(score)
}


func GetUserEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("content-type", "application/json")

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	collectionUser := client.Database("GoodJobGames").Collection("User")
	
	sliceLength := 100000
	params := mux.Vars(request)
	id := params["guid"]

	var user User
	err := collectionUser.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
		return
	}
	// e.g. here if user rank = 100,000 , can find actual rank in document whose slice id = 1. If it was 100,001 , then slice id = 2.
	sliceID := (user.Rank - 1) / sliceLength + 1

	rankReplacer := GetRankReplacer(sliceID)
	if user.Rank % sliceLength == 0 {
		user.Rank = rankReplacer.UserToActual[sliceLength]
	} else {
		user.Rank = rankReplacer.UserToActual[user.Rank % sliceLength]
	}

	response.WriteHeader(200)
	json.NewEncoder(response).Encode(user)
}


func CreateUserEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("content-type", "application/json")
	
	var user User
	_ = json.NewDecoder(request.Body).Decode(&user)

	id := uuid.New()
	user.ID = id.String()

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	collectionUser := client.Database("GoodJobGames").Collection("User")
	collectionRankReplacer := client.Database("GoodJobGames").Collection("RankReplacer")

	sliceLength := 100000
	count, _ := collectionUser.CountDocuments(ctx, bson.M{})

	// set the rank of user as the last
	user.Rank = int(count + 1)
	// e.g. here if user rank = 100,000 , can find actual rank in document whose slice id = 1. If it was 100,001 , then slice id = 2.
	sliceID := (user.Rank - 1) / sliceLength + 1
	
	// when the user rank should be placed in a new document after 100,000 entry is completed in previous document
	if user.Rank % sliceLength == 1 { 
		var rankReplacer RankReplacer

		rankReplacer.SliceID = sliceID
		rankReplacer.UserToActual = append(rankReplacer.UserToActual, 0) // First index is dummy
		rankReplacer.UserToActual = append(rankReplacer.UserToActual, user.Rank)
		
		rankReplacer.ActualToUser = append(rankReplacer.ActualToUser, 0) // First index is dummy
		rankReplacer.ActualToUser = append(rankReplacer.ActualToUser, user.Rank)

		_, err := collectionRankReplacer.InsertOne(ctx, rankReplacer)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}
	} else { // when the last document does not have 100,000 yet, then add current rank to last document's arrays
		rankReplacer := GetRankReplacer(sliceID)
		
		rankReplacer.UserToActual = append(rankReplacer.UserToActual, user.Rank)
		rankReplacer.ActualToUser = append(rankReplacer.ActualToUser, user.Rank)
		
		_, err := collectionRankReplacer.UpdateOne(
			ctx,
			bson.M{"slice_id": sliceID},
			bson.M{"$set": bson.M{"user_to_actual": rankReplacer.UserToActual, "actual_to_user": rankReplacer.ActualToUser}},
		)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}
	}

	// find the actual rank of the user and update necessary ranks in corresponding documents of RankReplacer
	if user.Rank != 1 {
		initialActualRank := user.Rank
		FindRankAndUpdate(user, initialActualRank)
	}
	
	// insert the user
	_, err := collectionUser.InsertOne(ctx, user)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
		return
	}

	rankReplacer := GetRankReplacer(sliceID)
	if user.Rank % sliceLength == 0 {
		user.Rank = rankReplacer.UserToActual[sliceLength]
	} else {
		user.Rank = rankReplacer.UserToActual[user.Rank % sliceLength]
	}
	
	response.WriteHeader(201)
	json.NewEncoder(response).Encode(user)
}



func AddUsersEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("content-type", "application/json")

	type  UserNumber struct {
		Number   int `json:"number_of_users"`
		MaxPoint int `json:"max_point"`
	}
	var numberOfUsers UserNumber
	_ = json.NewDecoder(request.Body).Decode(&numberOfUsers)

	collectionUser := client.Database("GoodJobGames").Collection("User")
	collectionRankReplacer := client.Database("GoodJobGames").Collection("RankReplacer")

	var countryCodes []string
	countryCodes = append(countryCodes, "tr")
	countryCodes = append(countryCodes, "us")

	name := "gjg_"

	count, _ := collectionUser.CountDocuments(nil, bson.M{})
	rankUser := int(count + 1)

	// START inserting random users
	for i := 1; i <= numberOfUsers.Number; i++ {
		var user User
		
		id := uuid.New()
		user.ID = id.String()
		user.DisplayName = name + strconv.Itoa(i)
		
		rand.Seed(time.Now().UnixNano())
		user.Points = float64(rand.Intn(numberOfUsers.MaxPoint))
		user.Country = countryCodes[rand.Intn(2)]

		user.Rank = rankUser
		rankUser++
		
		collectionUser.InsertOne(nil, user)
	}
	lastRankUser := rankUser - 1
	// END inserting random users

	// START find all users and sort them
	cursor, _ := collectionUser.Find(nil, bson.D{})

	var users []User
	defer cursor.Close(nil)
	for i := 1; cursor.Next(nil); i++ {
		var user User
		cursor.Decode(&user)
		users = append(users, user)
	}

	sort.Slice(users, func(i, j int) bool {
		return (users[i].Points > users[j].Points) || (users[i].Points == users[j].Points && users[i].DisplayName < users[j].DisplayName)
	})
	// END find all users and sort them

	// START inserting to or updating RankReplacer
	sliceLength := 100000
	lastSliceID := (lastRankUser - 1) / sliceLength + 1
	var rankReplacerSlices []RankReplacer
	for sliceID := 1; sliceID <= lastSliceID; sliceID++ {
		var rankReplacer RankReplacer
		rankReplacer.SliceID = sliceID
		if sliceID == lastSliceID {
			if lastRankUser % sliceLength == 0 {
				rankReplacer.UserToActual = make([]int, sliceLength + 1)
				rankReplacer.ActualToUser = make([]int, sliceLength + 1)
			} else {
				rankReplacer.UserToActual = make([]int, (lastRankUser % sliceLength) + 1)
				rankReplacer.ActualToUser = make([]int, (lastRankUser % sliceLength) + 1)
			}			
		} else {
			rankReplacer.UserToActual = make([]int, sliceLength + 1)
			rankReplacer.ActualToUser = make([]int, sliceLength + 1)
		}
		rankReplacerSlices = append(rankReplacerSlices, rankReplacer)
	}
	
	var rankActual int
	var sliceID int
	for i, user := range users {
		rankActual = i + 1
		rankUser = user.Rank

		sliceID = (rankActual - 1) / sliceLength + 1
		if rankActual % sliceLength == 0 {
			rankReplacerSlices[sliceID-1].ActualToUser[sliceLength] = rankUser
		} else {
			rankReplacerSlices[sliceID-1].ActualToUser[rankActual % sliceLength] = rankUser
		}
		
		sliceID = (rankUser - 1) / sliceLength + 1
		if rankUser % sliceLength == 0 {
			rankReplacerSlices[sliceID-1].UserToActual[sliceLength] = rankActual
		} else {
			rankReplacerSlices[sliceID-1].UserToActual[rankUser % sliceLength] = rankActual
		}
	}

	count, _ = collectionRankReplacer.CountDocuments(nil, bson.M{})
	for sliceID := 1; sliceID <= lastSliceID; sliceID++ {
		if sliceID <= int(count) {
			// update RankReplacer document
			collectionRankReplacer.UpdateOne(
				nil,
				bson.M{"slice_id": sliceID},
				bson.M{"$set": bson.M{"user_to_actual": rankReplacerSlices[sliceID-1].UserToActual, "actual_to_user": rankReplacerSlices[sliceID-1].ActualToUser}},
			)
		} else {
			collectionRankReplacer.InsertOne(nil, rankReplacerSlices[sliceID-1])
		}
	}
	// END inserting to or updating RankReplacer
	response.WriteHeader(201)
	response.Write([]byte(`{ "message": "` + strconv.Itoa(numberOfUsers.Number) + ` users have been inserted successfully" }`))
}



//------------------------ ENDPOINTS END HERE ------------------------
