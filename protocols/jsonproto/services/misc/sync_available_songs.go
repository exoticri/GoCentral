package misc

import (
	"log"
	"rb3server/protocols/jsonproto/marshaler"

	"github.com/ihatecompvir/nex-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type MiscSyncAvailableSongsRequest struct {
	Region      string `json:"region"`
	SystemMS    int    `json:"system_ms"`
	MachineID   string `json:"machine_id"`
	SessionGUID string `json:"session_guid"`
	PIDs        []int  `json:"pidXXX"`
	SIDs        string `json:"sids"`
	USIDs       string `json:"usids"`
}

type MiscSyncAvailableSongsResponse struct {
	RetCode int `json:"ret_code"`
}

type MiscSyncAvailableSongsService struct {
}

func (service MiscSyncAvailableSongsService) Path() string {
	return "misc/sync_available_songs"
}

func (service MiscSyncAvailableSongsService) Handle(data string, database *mongo.Database, client *nex.Client) (string, error) {
	var req MiscSyncAvailableSongsRequest
	err := marshaler.UnmarshalRequest(data, &req)
	if err != nil {
		return "", err
	}

	if req.PIDs[0] != int(client.PlayerID()) {
		log.Println("Client-supplied PID did not match server-assigned PID, rejecting songlist sync")
		return "", err
	}

	usersCollection := database.Collection("users")

	for _, pid := range req.PIDs {
		// update sids and usids fields on user with pid
		_, err := usersCollection.UpdateOne(nil, bson.M{"pid": pid}, bson.M{"$set": bson.M{"sids": req.SIDs, "usids": req.USIDs}})

		if err != nil {
			log.Printf("Could not update songlist for PID %v: %v\n", pid, err)
			return marshaler.MarshalResponse(service.Path(), []MiscSyncAvailableSongsResponse{{0}})
		}
	}

	return marshaler.MarshalResponse(service.Path(), []MiscSyncAvailableSongsResponse{{1}})
}
