package bungie

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// ClientPool is a simple client buffer that will provided round robin access to a collection of Clients.
type ClientPool struct {
	Clients []*Client
	current int
}

// NewClientPool is a convenience initializer to create a new collection of Clients.
func NewClientPool() *ClientPool {

	addresses := readClientAddresses()
	clients := make([]*Client, 0, len(addresses))
	for _, addr := range addresses {
		client, err := NewCustomAddrClient(addr)
		if err != nil {
			fmt.Println("Error creating custom ipv6 client: ", err.Error())
			continue
		}

		clients = append(clients, client)
	}
	if len(clients) == 0 {
		clients = append(clients, &Client{Client: http.DefaultClient})
	}

	return &ClientPool{
		Clients: clients,
	}
}

// Get will return a pointer to the next Client that should be used.
func (pool *ClientPool) Get() *Client {
	c := pool.Clients[pool.current]
	if pool.current == (len(pool.Clients) - 1) {
		pool.current = 0
	} else {
		pool.current++
	}

	return c
}

func readClientAddresses() []string {
	// TODO: This should come from the environment or a file
	return []string{
		"2604:a880:1:20::4274:b001",
		"2604:a880:1:20::4274:b002",
		"2604:a880:1:20::4274:b003",
		"2604:a880:1:20::4274:b004",
		"2604:a880:1:20::4274:b005",
		"2604:a880:1:20::4274:b006",
		"2604:a880:1:20::4274:b007",
		"2604:a880:1:20::4274:b008",
		"2604:a880:1:20::4274:b009",
		"2604:a880:1:20::4274:b00a",
		"2604:a880:1:20::4274:b00b",
		"2604:a880:1:20::4274:b00c",
		"2604:a880:1:20::4274:b00d",
		"2604:a880:1:20::4274:b00e",
	}
}

// Client is a type that contains all information needed to make requests to the
// Bungie API.
type Client struct {
	*http.Client
	AccessToken string
	APIToken    string
}

// NewCustomAddrClient will create a new Bungie Client instance with the provided local IP address.
func NewCustomAddrClient(address string) (*Client, error) {

	//ipv6 := fmt.Sprintf("%s::%x", os.Getenv("IPV6_ADDR"), rand.Intn(ipv6Count))
	localAddr, err := net.ResolveIPAddr("ip6", address)
	if err != nil {
		return nil, err
	}

	localTCPAddr := net.TCPAddr{
		IP: localAddr.IP,
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			LocalAddr: &localTCPAddr,
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
	}

	httpClient := &http.Client{Transport: transport}

	return &Client{Client: httpClient}, nil
}

func (c *Client) AddAuthValues(accessToken, apiKey string) {
	c.APIToken = apiKey
	c.AccessToken = accessToken
}

// AddAuthHeadersToRequest will handle adding the authentication headers from the
// current client to the specified Request.
func (c *Client) AddAuthHeadersToRequest(req *http.Request) {
	for key, val := range c.AuthenticationHeaders() {
		req.Header.Add(key, val)
	}
}

// AuthenticationHeaders will generate a map with the required headers to make
// an authenticated HTTP call to the Bungie API.
func (c *Client) AuthenticationHeaders() map[string]string {
	return map[string]string{
		"X-Api-Key":     c.APIToken,
		"Authorization": "Bearer " + c.AccessToken,
	}
}

// GetCurrentAccount will request the user info for the current user
// based on the OAuth token provided as part of the request.
func (c *Client) GetCurrentAccount() (*GetAccountResponse, error) {

	req, _ := http.NewRequest("GET", GetCurrentAccountEndpoint, nil)
	req.Header.Add("Content-Type", "application/json")
	c.AddAuthHeadersToRequest(req)

	itemsResponse, err := c.Do(req)
	if err != nil {
		fmt.Println("Failed to read the Items response from Bungie!: ", err.Error())
		return nil, err
	}
	defer itemsResponse.Body.Close()

	accountResponse := GetAccountResponse{}
	json.NewDecoder(itemsResponse.Body).Decode(&accountResponse)

	return &accountResponse, nil
}

// GetUserItems will make a request to the bungie API and retrieve all of the
// items for a specific Destiny membership ID. This includes all of their characters
// as well as the vault. The vault with have a character index of -1.
func (c *Client) GetUserItems(membershipType int, membershipID string) (*ItemsEndpointResponse, error) {
	endpoint := fmt.Sprintf(ItemsEndpointFormat, membershipType, membershipID)

	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Add("Content-Type", "application/json")
	c.AddAuthHeadersToRequest(req)

	itemsResponse, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer itemsResponse.Body.Close()

	itemsJSON := &ItemsEndpointResponse{}
	json.NewDecoder(itemsResponse.Body).Decode(&itemsJSON)

	return itemsJSON, nil
}

// PostTransferItem is responsible for calling the Bungie.net API to transfer
// an item from a source to a destination. This could be either a user's character
// or the vault.
func (c *Client) PostTransferItem(body map[string]interface{}) {

	// TODO: This retry logic should probably be added to a middleware type function
	retry := true
	attempts := 0
	for {
		retry = false
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", TransferItemEndpointURL, strings.NewReader(string(jsonBody)))
		req.Header.Add("Content-Type", "application/json")
		c.AddAuthHeadersToRequest(req)

		resp, err := c.Do(req)
		if err != nil {
			fmt.Println("Error transferring item: ", err.Error())
			return
		}
		defer resp.Body.Close()

		var response BaseResponse
		json.NewDecoder(resp.Body).Decode(&response)
		if response.ErrorCode == 36 || response.ErrorStatus == "ThrottleLimitExceededMomentarily" {
			time.Sleep(1 * time.Second)
			retry = true
		}

		fmt.Printf("Response for transfer request: %+v\n", response)
		attempts++
		if retry == false || attempts >= 5 {
			break
		}
	}
}

// PostEquipItem is responsible for calling the Bungie.net API to equip
// an item on a specific character.
func (c *Client) PostEquipItem(body map[string]interface{}) {

	// TODO: This retry logic should probably be added to a middleware type function
	retry := true
	attempts := 0
	for {
		retry = false
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", EquipItemEndpointURL, strings.NewReader(string(jsonBody)))
		req.Header.Add("Content-Type", "application/json")
		c.AddAuthHeadersToRequest(req)

		resp, err := c.Do(req)
		if err != nil {
			fmt.Println("Error equipping item: ", err.Error())
			return
		}
		defer resp.Body.Close()

		var response BaseResponse
		json.NewDecoder(resp.Body).Decode(&response)
		if response.ErrorCode == 36 || response.ErrorStatus == "ThrottleLimitExceededMomentarily" {
			time.Sleep(1 * time.Second)
			retry = true
		}

		fmt.Printf("Response for equip request: %+v\n", response)
		attempts++
		if retry == false || attempts >= 5 {
			break
		}
	}
}
