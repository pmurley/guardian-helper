package trials

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"time"

	"strconv"

	"bytes"

	"github.com/mikeflynn/go-alexa/skillserver"
	"github.com/rking788/guardian-helper/bungie"
	"github.com/rking788/guardian-helper/db"
)

// CurrentMap represents the metadata describing the current active map in Trials of Osiris
type CurrentMap struct {
	Name       string `json:"activityName"`
	WeekNumber string `json:"week"`
	StartDate  string `json:"start_date"`
}

// CurrentWeek represents the stats for the current week for a particular player (specified by membershipId)
type CurrentWeek struct {
	Matches string `json:"matches"`
	Losses  string `json:"losses"`
	KD      string `json:"kd"`
}

// WeaponUsage is used in the response from the weapon percentage endpoint. It describes the popularity
// of a specific weapon (not type).
type WeaponUsage struct {
	Name           string `json:"name"`
	BucketTypeHash string `json:"bucketTypeHash"`
	Percentage     string `json:"percentage"`
	Tier           string `json:"tier"`
}

type PersonalWeaponStats struct {
	Headshots    int    `json:"headshots"`
	TotalMatches int    `json:"total_matches"`
	Kills        int    `json:"kills"`
	WeaponID     string `json:"weaponId"`
}

// GetCurrentMap will make a request to the Trials Report API endpoint and
// return an Alexa response describing the current map.
func GetCurrentMap() (*skillserver.EchoResponse, error) {

	response := skillserver.NewEchoResponse()

	currentMap, err := requestCurrentMap()
	start, err := time.Parse("2006-01-02 15:04:05", currentMap.StartDate)
	if err != nil {
		fmt.Println("Failed to read the current map from Trials Report!: ", err.Error())
		return nil, err
	}

	response.OutputSpeech(fmt.Sprintf("According to Trials Report, the current Trials of Osiris map beginning %s %d is %s, goodluck Guardian.", start.Month().String(), start.Day(), currentMap.Name))

	return response, nil
}

// Convenience method for loading current map data from Trials Report. This is used in a
// few different spots, mostly for the current week number.
func requestCurrentMap() (*CurrentMap, error) {
	req, _ := http.NewRequest("GET", TrialsCurrentMapEndpoint, nil)
	req.Header.Add("Content-Type", "application/json")

	mapResponse, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer mapResponse.Body.Close()

	currentMaps := make([]CurrentMap, 0, 1)
	err = json.NewDecoder(mapResponse.Body).Decode(&currentMaps)
	if err != nil {
		return nil, err
	} else if len(currentMaps) <= 0 {
		return nil, errors.New("Error got an empty current map array back from trials report")
	}

	return &currentMaps[0], nil
}

// GetCurrentWeek is responsible for requesting the players stats from the current week from Trials Report.
func GetCurrentWeek(token string) (*skillserver.EchoResponse, error) {
	response := skillserver.NewEchoResponse()

	membershipID, err := findMembershipID(token)

	url := fmt.Sprintf(TrialsCurrentWeekEndpointFmt, membershipID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")

	mapResponse, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Failed to read the current week stats response from Trials Report!: ", err.Error())
		return nil, err
	}
	defer mapResponse.Body.Close()

	currentWeeks := make([]CurrentWeek, 0, 1)
	err = json.NewDecoder(mapResponse.Body).Decode(&currentWeeks)
	if err != nil {
		fmt.Println("Error parsing trials report response: ", err.Error())
		return nil, err
	}

	matches, _ := strconv.ParseInt(currentWeeks[0].Matches, 10, 32)
	if matches != 0 {
		losses, _ := strconv.ParseInt(currentWeeks[0].Losses, 10, 32)
		wins := matches - losses
		kd := currentWeeks[0].KD
		response.OutputSpeech(fmt.Sprintf("So far you have played %d matches with %d wins, %d losses and a combined KD of %s, according to Trials Report", matches, wins, losses, kd))
	} else {
		response.OutputSpeech("You have not yet played any Trials of Osiris matches this week guardian.")
	}

	return response, nil
}

// findMembershipID is a helper function for loading the membership ID from the currently
// linked account, this eventually should take platform into account.
func findMembershipID(token string) (string, error) {

	client := bungie.NewClient(token, os.Getenv("BUNGIE_API_KEY"))
	currentAccount, err := client.GetCurrentAccount()
	if err != nil {
		fmt.Println("Error loading current account info from Bungie.net: ", err.Error())
		return "", err
	} else if currentAccount.Response == nil || currentAccount.Response.DestinyAccounts == nil ||
		len(currentAccount.Response.DestinyAccounts) == 0 {
		return "", errors.New("No linked Destiny account found on Bungie.net")
	}

	// TODO: This should take the platform into account instead of just defaulting to the first one.
	return currentAccount.Response.DestinyAccounts[0].UserInfo.MembershipID, nil
}

// GetWeaponUsagePercentages will return a response describing the top 3 used weapons
// by all players for the current week.
func GetWeaponUsagePercentages() (*skillserver.EchoResponse, error) {
	response := skillserver.NewEchoResponse()

	currentMap, err := requestCurrentMap()
	if err != nil {
		fmt.Println("Error loading current map from Trials Report: ", err.Error())
		return nil, err
	}

	url := fmt.Sprintf(TrialsWeaponPercentageEndpointFmt, currentMap.WeekNumber)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")

	weaponResponse, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error sending weapon percentages request to Trial Report: ", err.Error())
		return nil, err
	}
	defer weaponResponse.Body.Close()

	usages := make([]WeaponUsage, 0, 50)
	err = json.NewDecoder(weaponResponse.Body).Decode(&usages)

	buffer := bytes.NewBufferString("According to Trials Report, the top weapons used in trials this week are: ")
	// TODO: Maybe it would be good to have the user specify the number of top weapons they want returned.
	for i := 0; i < TopWeaponUsageLimit; i++ {
		usagePercent, _ := strconv.ParseFloat(usages[i].Percentage, 64)
		buffer.WriteString(fmt.Sprintf("%s with %.1f%%, ", usages[i].Name, usagePercent))
	}

	response.OutputSpeech(buffer.String())
	return response, nil
}

// GetPersonalTopWeapons will return a summary of the top weapons used by the linked player/account.
func GetPersonalTopWeapons(token string) (*skillserver.EchoResponse, error) {
	response := skillserver.NewEchoResponse()

	membershipID, err := findMembershipID(token)
	if err != nil {
		fmt.Println("Error loading membership ID for linked account: ", err.Error())
		return nil, err
	}

	url := fmt.Sprintf(TrialsTopWeaponsEndpointFmt, membershipID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")

	topWeaponsResponse, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error sending weapon percentages request to Trial Report: ", err.Error())
		return nil, err
	}
	defer topWeaponsResponse.Body.Close()

	usages := make([]PersonalWeaponStats, 0, 10)
	err = json.NewDecoder(topWeaponsResponse.Body).Decode(&usages)

	if len(usages) <= 0 {
		response.OutputSpeech("You have no top used weapons in Trials of Osiris")
		return response, nil
	}

	buffer := bytes.NewBufferString("According to Trials Report, your top weapons by kills are: ")
	for index, usage := range usages {

		if index >= TopWeaponUsageLimit {
			break
		}

		name, err := db.GetItemNameFromHash(usage.WeaponID)
		if err != nil {
			name = "Unknown"
		}

		buffer.WriteString(fmt.Sprintf("%s, ", name))
	}

	response.OutputSpeech(buffer.String())

	return response, nil
}
