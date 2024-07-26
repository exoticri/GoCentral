package servers

import (
	"context"
	"fmt"
	"log"
	"rb3server/database"
	"rb3server/models"
	"rb3server/quazal"
	"regexp"

	"github.com/ihatecompvir/nex-go"
	nexproto "github.com/ihatecompvir/nex-protocols-go"
	"go.mongodb.org/mongo-driver/bson"
)

// also handles Xbox 360 account switching
func NintendoCreateAccount(err error, client *nex.Client, callID uint32, username string, key string, groups uint32, email string) {

	rmcResponseStream := nex.NewStream()

	users := database.GocentralDatabase.Collection("users")
	configCollection := database.GocentralDatabase.Collection("config")
	var user models.User

	var ctype int // 0 = xbox360, 1= ps3, 2 = wii

	// Look for 'DummyNintendo' in the email address, if we find it, its a Wii console
	var rgx = regexp.MustCompile(`DummyNintendo`)
	res := rgx.FindStringSubmatch(email)

	if len(res) != 0 {
		ctype = 2
	}

	var config models.Config
	err = configCollection.FindOne(context.TODO(), bson.M{}).Decode(&config)
	if err != nil {
		log.Printf("Could not get config %v\n", err)
	}

	// Create a new user if not currently registered.
	if result := users.FindOne(nil, bson.M{"username": username}).Decode(&user); result != nil {
		log.Printf("%s has never connected before - create DB entry\n", username)

		guid, err := generateGUID()

		_, err = users.InsertOne(nil, bson.D{
			{Key: "username", Value: username},
			{Key: "pid", Value: config.LastPID + 1},
			{Key: "console_type", Value: ctype},
			{Key: "guid", Value: guid},
			{Key: "created_by_machine_id", Value: client.PlayerID()},
		})

		if err != nil {
			log.Printf("Could not create Nintendo user %s: %s\n", username, err)
			SendErrorCode(SecureServer, client, nexproto.AccountManagementProtocolID, callID, quazal.OperationError)
			return
		}

		_, err = configCollection.UpdateOne(
			nil,
			bson.M{},
			bson.D{
				{"$set", bson.D{{"last_pid", config.LastPID + 1}}},
			},
		)
		if err != nil {
			log.Println("Could not update config in database: ", err)
			SendErrorCode(SecureServer, client, nexproto.AccountManagementProtocolID, callID, quazal.OperationError)
			return
		}

		Config.LastPID = config.LastPID + 1
		Config.LastMachineID = config.LastMachineID
		Config.LastBandID = config.LastBandID
		Config.LastSetlistID = config.LastSetlistID
		Config.LastCharacterID = config.LastCharacterID

		// make sure we actually set the server-assigned PID to the new one when it is created
		client.SetPlayerID(user.PID)

		if err = users.FindOne(nil, bson.M{"username": username}).Decode(&user); err != nil {

			if err != nil {
				log.Printf("Could not find newly created Nintendo user: %s\n", err)
				SendErrorCode(SecureServer, client, nexproto.AccountManagementProtocolID, callID, quazal.OperationError)
				return
			}
		}
	}

	log.Printf("%s requesting Nintendo log in from Wii Friend Code %s, has PID %v\n", username, client.WiiFC, user.PID)

	client.Username = username

	// since the Wii doesn't try hitting RegisterEx after logging in, we have to set station URLs here
	// TODO: do this better / do this proper (there's gotta be a better way), find out how to set int_station_url
	newRVCID := uint32(SecureServer.ConnectionIDCounter().Increment())
	var stationURL string = "prudp:/address=" + client.Address().IP.String() + ";port=" + fmt.Sprint(client.Address().Port) + ";PID=" + fmt.Sprint(user.PID) + ";sid=15;type=3;RVCID=" + fmt.Sprint(newRVCID)

	client.SetExternalStationURL(stationURL)
	client.SetConnectionID(uint32(newRVCID))
	client.SetPlayerID(user.PID)

	// update station URL
	result, err := users.UpdateOne(
		nil,
		bson.M{"username": client.Username},
		bson.D{
			{"$set", bson.D{{"station_url", stationURL}}},
			{"$set", bson.D{{"int_station_url", ""}}},
		},
	)

	if err != nil {
		log.Printf("Could not update station URLs for Nintendo user %s: %s\n", username, err)
		SendErrorCode(SecureServer, client, nexproto.AccountManagementProtocolID, callID, quazal.OperationError)
		return
	}

	log.Printf("Updated %v station URL for %s \n", result.ModifiedCount, client.Username)

	rmcResponseStream.WriteUInt32LE(user.PID)

	rmcResponseBody := rmcResponseStream.Bytes()

	rmcResponse := nex.NewRMCResponse(nexproto.AccountManagementProtocolID, callID)
	rmcResponse.SetSuccess(nexproto.NintendoCreateAccount, rmcResponseBody)

	rmcResponseBytes := rmcResponse.Bytes()

	responsePacket, _ := nex.NewPacketV0(client, nil)

	responsePacket.SetVersion(0)
	responsePacket.SetSource(0x31)
	responsePacket.SetDestination(0x3F)
	responsePacket.SetType(nex.DataPacket)

	responsePacket.SetPayload(rmcResponseBytes)

	responsePacket.AddFlag(nex.FlagNeedsAck)
	responsePacket.AddFlag(nex.FlagReliable)

	SecureServer.Send(responsePacket)
}
