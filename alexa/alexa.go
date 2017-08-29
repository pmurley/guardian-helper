package alexa

// TODO: This file really needs a refactor. Endpoints that require a linked account
// should use some kind of middleware instead of having the check in individually handlers.

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/rking788/guardian-helper/bungie"
	"github.com/rking788/guardian-helper/trials"

	"strings"

	"github.com/garyburd/redigo/redis"
	"github.com/mikeflynn/go-alexa/skillserver"
)

// Session is responsible for storing information related to a specific skill invocation.
// A session will remain open if the LaunchRequest was received.
type Session struct {
	ID                   string
	Action               string
	ItemName             string
	DestinationClassHash int
	SourceClassHash      int
	Quantity             int
}

var (
	redisConnPool = newRedisPool(os.Getenv("REDIS_URL"))
)

// Redis related functions

func newRedisPool(addr string) *redis.Pool {
	// 25 is the maximum number of active connections for the Heroku Redis free tier
	return &redis.Pool{
		MaxIdle:     3,
		MaxActive:   25,
		IdleTimeout: 240 * time.Second,
		Dial:        func() (redis.Conn, error) { return redis.DialURL(addr) },
	}
}

// GetSession will attempt to read a session from the cache, if an existing one is not found, an empty session
// will be created with the specified sessionID.
func GetSession(sessionID string) (session *Session) {
	session = &Session{ID: sessionID}

	conn := redisConnPool.Get()
	defer conn.Close()

	key := fmt.Sprintf("sessions:%s", sessionID)
	reply, err := redis.String(conn.Do("GET", key))
	if err != nil {
		// NOTE: This is a normal situation, if the session is not stored in the cache, it will hit this condition.
		return
	}

	err = json.Unmarshal([]byte(reply), session)

	return
}

// SaveSession will persist the given session to the cache. This will allow support for long running
// Alexa sessions that continually prompt the user for more information.
func SaveSession(session *Session) {

	conn := redisConnPool.Get()
	defer conn.Close()

	sessionBytes, err := json.Marshal(session)
	if err != nil {
		fmt.Println("Couldn't marshal session to string: ", err.Error())
		return
	}

	key := fmt.Sprintf("sessions:%s", session.ID)
	_, err = conn.Do("SET", key, string(sessionBytes))
	if err != nil {
		fmt.Println("Failed to set session: ", err.Error())
	}
}

// ClearSession will remove the specified session from the local cache, this will be done
// when the user completes a full request session.
func ClearSession(sessionID string) {

	conn := redisConnPool.Get()
	defer conn.Close()

	key := fmt.Sprintf("sessions:%s", sessionID)
	_, err := conn.Do("DEL", key)
	if err != nil {
		fmt.Println("Failed to delete the session from the Redis cache: ", err.Error())
	}
}

// WelcomePrompt is responsible for prompting the user with information about what they can ask
// the skill to do.
func WelcomePrompt(echoRequest *skillserver.EchoRequest) (response *skillserver.EchoResponse) {
	response = skillserver.NewEchoResponse()

	response.OutputSpeech("Welcome Guardian, would you like to transfer an item to a specific character, find out how many of an item you have, or ask about Trials of Osiris?").
		Reprompt("Do you want to transfer an item, find out how much of an item you have, or ask about Trials of Osiris?").
		EndSession(false)

	return
}

// HelpPrompt provides the required information to satisfy the HelpIntent built-in Alexa intent. This should
// provider information to the user to let them know what the skill can do without providing exact commands.
func HelpPrompt(echoRequest *skillserver.EchoRequest) (response *skillserver.EchoResponse) {
	response = skillserver.NewEchoResponse()

	response.OutputSpeech("Welcome Guardian, I am here to help manage your Destiny in-game inventory. You can ask " +
		"me to transfer items between any of your available characters including the vault. You can also ask how many of an " +
		"item you have. Trials of Osiris statistics provided by Trials Report are available too.").
		EndSession(false)

	return
}

// CountItem calls the Bungie API to see count the number of Items on all characters and
// in the vault.
func CountItem(echoRequest *skillserver.EchoRequest) (response *skillserver.EchoResponse) {

	accessToken := echoRequest.Session.User.AccessToken
	if accessToken == "" {
		response = skillserver.NewEchoResponse()
		response.
			OutputSpeech("Sorry Guardian, it looks like your Bungie.net account needs to be linked in the Alexa app.").
			LinkAccountCard()
		return
	}

	item, _ := echoRequest.GetSlotValue("Item")
	lowerItem := strings.ToLower(item)
	response, err := bungie.CountItem(lowerItem, accessToken)
	if err != nil {
		fmt.Println("Error counting the number of items: ", err.Error())
		response = skillserver.NewEchoResponse()
		response.OutputSpeech("Sorry Guardian, an error occurred counting that item.")
	}

	return
}

// TransferItem will attempt to transfer either a specific quantity or all of a
// specific item to a specified character. The item name and destination are the
// required fields. The quantity and source are optional.
func TransferItem(request *skillserver.EchoRequest) (response *skillserver.EchoResponse) {

	accessToken := request.Session.User.AccessToken
	if accessToken == "" {
		response = skillserver.NewEchoResponse()
		response.
			OutputSpeech("Sorry Guardian, it looks like your Bungie.net account needs to be linked in the Alexa app.").
			LinkAccountCard()
		return
	}

	countStr, _ := request.GetSlotValue("Count")
	count := -1
	if countStr != "" {
		tempCount, ok := strconv.Atoi(countStr)
		if ok != nil {
			response = skillserver.NewEchoResponse()
			response.OutputSpeech("Sorry Guardian, I didn't understand the number you asked to be transferred. Do not specify a quantity if you want all to be transferred.")
			return
		}

		if tempCount <= 0 {
			output := fmt.Sprintf("Sorry Guardian, you need to specify a positive, non-zero number to be transferred, not %d", tempCount)
			fmt.Println(output)
			response.OutputSpeech(output)
			return
		}

		count = tempCount
	}

	item, _ := request.GetSlotValue("Item")
	sourceClass, _ := request.GetSlotValue("Source")
	destinationClass, _ := request.GetSlotValue("Destination")
	if destinationClass == "" {
		response = skillserver.NewEchoResponse()
		response.OutputSpeech("Sorry Guardian, you must specify a destination for the items to be transferred.")
		return
	}

	output := fmt.Sprintf("Transferring %d of your %s from your %s to your %s", count, strings.ToLower(item), strings.ToLower(sourceClass), strings.ToLower(destinationClass))
	fmt.Println(output)
	response, err := bungie.TransferItem(strings.ToLower(item), accessToken, strings.ToLower(sourceClass), strings.ToLower(destinationClass), count)
	if err != nil {
		response = skillserver.NewEchoResponse()
		response.OutputSpeech("Sorry Guardian, an error occurred trying to transfer that item.")
		return
	}

	return
}

func MaxLight(request *skillserver.EchoRequest) (response *skillserver.EchoResponse) {

	accessToken := request.Session.User.AccessToken
	if accessToken == "" {
		response = skillserver.NewEchoResponse()
		response.
			OutputSpeech("Sorry Guardian, it looks like your Bungie.net account needs to be linked in the Alexa app.").
			LinkAccountCard()
		return
	}

	response, err := bungie.EquipMaxLightGear(accessToken)
	if err != nil {
		fmt.Println("Error occurred equipping max light: ", err.Error())
		response = skillserver.NewEchoResponse()
		response.OutputSpeech("Sorry Guardian, an error occurred equipping your max light gear.")
	}

	return
}

// UnloadEngrams will take all engrams on all of the current user's characters and transfer them all to the
// vault to allow the player to continue farming.
func UnloadEngrams(request *skillserver.EchoRequest) (response *skillserver.EchoResponse) {

	accessToken := request.Session.User.AccessToken
	if accessToken == "" {
		response = skillserver.NewEchoResponse()
		response.
			OutputSpeech("Sorry Guardian, it looks like your Bungie.net account needs to be linked in the Alexa app.").
			LinkAccountCard()
		return
	}

	response, err := bungie.UnloadEngrams(accessToken)
	if err != nil {
		fmt.Println("Error occurred unloading engrams: ", err.Error())
		response = skillserver.NewEchoResponse()
		response.OutputSpeech("Sorry Guardian, an error occurred moving your engrams.")
	}

	return
}

/*
 * Trials of Osiris data
 */

// CurrentTrialsMap will return a brief description of the current map in the active Trials of Osiris week.
func CurrentTrialsMap(request *skillserver.EchoRequest) (response *skillserver.EchoResponse) {

	response, err := trials.GetCurrentMap()
	if err != nil {
		response = skillserver.NewEchoResponse()
		response.OutputSpeech("Sorry Guardian, I cannot access this information right now, please try again later.")
		return
	}

	return
}

// CurrentTrialsWeek will return a brief description of the current map in the active Trials of Osiris week.
func CurrentTrialsWeek(request *skillserver.EchoRequest) (response *skillserver.EchoResponse) {

	accessToken := request.Session.User.AccessToken
	if accessToken == "" {
		response = skillserver.NewEchoResponse()
		response.
			OutputSpeech("Sorry Guardian, it looks like your Bungie.net account needs to be linked in the Alexa app.").
			LinkAccountCard()
		return
	}

	response, err := trials.GetCurrentWeek(accessToken)
	if err != nil {
		response = skillserver.NewEchoResponse()
		response.OutputSpeech("Sorry Guardian, I cannot access this information right now, please try again later.")
		return
	}

	return
}

// PopularWeapons will check Trials Report for the most popular specific weapons for the current week.
func PopularWeapons(request *skillserver.EchoRequest) (response *skillserver.EchoResponse) {

	response, err := trials.GetWeaponUsagePercentages()
	if err != nil {
		response = skillserver.NewEchoResponse()
		response.OutputSpeech("Sorry Guardian, I cannot access this information at this time, please try again later")
		return
	}

	return
}

// PersonalTopWeapons will check Trials Report for the most used weapons for the current user.
func PersonalTopWeapons(request *skillserver.EchoRequest) (response *skillserver.EchoResponse) {

	accessToken := request.Session.User.AccessToken
	if accessToken == "" {
		response = skillserver.NewEchoResponse()
		response.
			OutputSpeech("Sorry Guardian, it looks like your Bungie.net account needs to be linked in the Alexa app.").
			LinkAccountCard()
		return
	}

	response, err := trials.GetPersonalTopWeapons(accessToken)
	if err != nil {
		response = skillserver.NewEchoResponse()
		response.OutputSpeech("Sorry Guardian, I cannot access this information at this time, please try again later")
		return
	}

	return
}

// PopularWeaponTypes will return info about what classes of weapons are getting
// the most kills in Trials of Osiris.
func PopularWeaponTypes() (response *skillserver.EchoResponse) {
	response, err := trials.GetPopularWeaponTypes()

	if err != nil {
		response = skillserver.NewEchoResponse()
		response.OutputSpeech("Sorry Guardian, I cannot access this information at this time, pleast try again later")
		return
	}

	return
}
