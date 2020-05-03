
package main


type RankReplacer struct {
	// after 100,000 ranking entry added for the arrays below, new RankReplacer object is generated with new slice ID
	SliceID      int   `bson:"slice_id,omitempty"`
	UserToActual []int `bson:"user_to_actual,omitempty"` // index = rank field of user, element = actual rank of user
	ActualToUser []int `bson:"actual_to_user,omitempty"` // index = actual rank of user, element = rank field of user
}


type User struct {
	ID          string  `json:"user_id,omitempty" bson:"_id,omitempty"`
	Rank        int     `json:"rank,omitempty" bson:"rank,omitempty"`
	Points      float64 `json:"points,omitempty" bson:"points,omitempty"`
	DisplayName string  `json:"display_name,omitempty" bson:"display_name,omitempty"`
	Country     string  `json:"country,omitempty" bson:"country,omitempty"`
}



