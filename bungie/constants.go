package bungie

// Constant API endpoints
const (
	AppAuthURL                        = "https://www.bungie.net/en/Application/Authorize/2579"
	TokensFromRefreshTokenURL         = "https://www.bungie.net/Platform/App/GetAccessTokensFromRefreshToken/"
	TokensFromAuthCodeURL             = "https://www.bungie.net/Platform/App/GetAccessTokensFromCode/"
	MembershipIDFromDisplayNameFormat = "http://www.bungie.net/Platform/Destiny/SearchDestinyPlayer/%d/%s/"
	ItemsEndpointFormat               = "http://www.bungie.net/Platform/Destiny/%d/Account/%s/Items"
)

// Hash values for different class types 'classHash' JSON key
const (
	WARLOCK = 2271682572
	TITAN   = 3655393761
	HUNTER  = 671679327
)

var classHashToName = map[uint]string{
	WARLOCK: "Warlock",
	TITAN:   "Titan",
	HUNTER:  "Hunter",
}

// Class Enum value passed in some of the Destiny API responses
const (
	TitanEnum        = 0
	HunterEnum       = 1
	WarlockEnum      = 2
	UnknownClassEnum = 3
)

// Hash values for Race types 'raceHash' JSON key
const (
	AWOKEN = 2803282938
	HUMAN  = 3887404748
	EXO    = 898834093
)

// Hash values for Gender 'genderHash' JSON key
const (
	MALE   = 3111576190
	FEMALE = 2204441813
)

// Gender Enum values used in some of the Bungie API responses
const (
	MaleEnum          = 0
	FemaleEnum        = 1
	UnknownGenderEnum = 2
)

// Membership type constant values
const (
	XBOX = 1
	PSN  = 2
)

// Alexa doesn't understand some of the dsetiny items or splits them into separate words
// This will allow us to translate to the correct name before doing the lookup.
var commonAlexaTranslations = map[string]string{
	"spin metal":    "spinmetal",
	"passage coins": "passage coin",
	"strange coins": "strange coin",
	"exotic shards": "exotic shard",
	"worm spore":    "wormspore",
}