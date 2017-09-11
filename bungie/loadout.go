package bungie

import (
	"fmt"
	"sort"
)

// Loadout will hold all items for a unique set of weapons, armor, ghost, class item, and artifact
type Loadout map[EquipmentBucket]*Item

func (l Loadout) calculateLightLevel() float64 {

	light := 0.0

	light += float64(l[Kinetic].Power()) * 0.143
	light += float64(l[Energy].Power()) * 0.143
	light += float64(l[Power].Power()) * 0.143
	//light += float64(l[Ghost].Power()) * 0.08

	light += float64(l[Helmet].Power()) * 0.119
	light += float64(l[Gauntlets].Power()) * 0.119
	light += float64(l[Chest].Power()) * 0.119
	light += float64(l[Legs].Power()) * 0.119
	light += float64(l[ClassArmor].Power()) * 0.095
	//light += float64(l[Artifact].Power()) * 0.08

	return light
}

func (l Loadout) toSlice() []*Item {

	result := make([]*Item, 0, ClassArmor-Kinetic)
	for i := Kinetic; i <= ClassArmor; i++ {
		result = append(result, l[i])
	}

	return result
}

func findMaxLightLoadout(profile *Profile, destinationID string) Loadout {
	// Start by filtering all items that are NOT exotics
	destinationClassType := profile.Characters.findCharacterFromID(destinationID).ClassType
	filteredItems := profile.AllItems.
		FilterItems(itemNotTierTypeFilter, ExoticTier).
		FilterItems(itemClassTypeFilter, destinationClassType)
	gearSortedByLight := groupAndSortGear(filteredItems)

	// Find the best loadout given just legendary weapons
	loadout := make(Loadout)
	for i := Kinetic; i <= ClassArmor; i++ {
		loadout[i] = findBestItemForBucket(i, gearSortedByLight[i], destinationID)
	}

	// Determine the best exotics to use for both weapons and armor
	exotics := profile.AllItems.
		FilterItems(itemTierTypeFilter, ExoticTier).
		FilterItems(itemClassTypeFilter, destinationClassType)
	exoticsSortedAndGrouped := groupAndSortGear(exotics)

	// Override inventory items with exotics as needed
	for _, bucket := range [3]EquipmentBucket{ClassArmor} {
		exoticCandidate := findBestItemForBucket(bucket, exoticsSortedAndGrouped[bucket], destinationID)
		if exoticCandidate != nil && exoticCandidate.Power() > loadout[bucket].Power() {
			fmt.Printf("Overriding %s...\n", bucket)
			loadout[bucket] = exoticCandidate
		}
	}

	var weaponExoticCandidate *Item
	var weaponBucket EquipmentBucket
	for _, bucket := range [3]EquipmentBucket{Kinetic, Energy, Power} {
		exoticCandidate := findBestItemForBucket(bucket, exoticsSortedAndGrouped[bucket], destinationID)
		if exoticCandidate != nil && exoticCandidate.Power() > loadout[bucket].Power() {
			if weaponExoticCandidate == nil || exoticCandidate.Power() > weaponExoticCandidate.Power() {
				weaponExoticCandidate = exoticCandidate
				weaponBucket = bucket
				fmt.Printf("Overriding %s...\n", bucket)
			}
		}
	}
	if weaponExoticCandidate != nil {
		loadout[weaponBucket] = weaponExoticCandidate
	}

	var armorExoticCandidate *Item
	var armorBucket EquipmentBucket
	for _, bucket := range [4]EquipmentBucket{Helmet, Gauntlets, Chest, Legs} {
		exoticCandidate := findBestItemForBucket(bucket, exoticsSortedAndGrouped[bucket], destinationID)
		if exoticCandidate != nil && exoticCandidate.Power() > loadout[bucket].Power() {
			if armorExoticCandidate == nil || exoticCandidate.Power() > armorExoticCandidate.Power() {
				armorExoticCandidate = exoticCandidate
				armorBucket = bucket
				fmt.Printf("Overriding %s...\n", bucket)
			}
		}
	}
	if armorExoticCandidate != nil {
		loadout[armorBucket] = armorExoticCandidate
	}

	return loadout
}

func equipLoadout(loadout Loadout, destinationID string, profile *Profile, membershipType int, client *Client) error {

	characters := profile.Characters
	// TODO: This should swap any items that are currently equipped on other characters
	// to prepare them to be transferred
	for bucket, item := range loadout {
		if item.TransferStatus == ItemIsEquipped && item.Character.CharacterID != destinationID {
			swapEquippedItem(item, profile, bucket, membershipType, client)
		}
	}

	// Move all items to the destination character
	err := moveLoadoutToCharacter(loadout, destinationID, characters, membershipType, client)
	if err != nil {
		fmt.Println("Error moving loadout to destination character: ", err.Error())
		return err
	}

	// Equip all items that were just transferred
	equipItems(loadout.toSlice(), destinationID, characters, membershipType, client)

	return nil
}

// swapEquippedItem is responsible for equipping a new item on a character that is not the destination
// of a transfer. This way it free up the item to be equipped by the desired character.
func swapEquippedItem(item *Item, profile *Profile, bucket EquipmentBucket, membershipType int, client *Client) {

	// TODO: Currently filtering out exotics to make it easier
	// This should be more robust. There is no guarantee the character already has an exotic
	// equipped in a different slot and this may be the only option to swap out this item.
	reverseLightSortedItems := profile.AllItems.
		FilterItems(itemCharacterIDFilter, item.CharacterID).
		FilterItems(itemBucketHashFilter, item.BucketHash).
		FilterItems(itemNotTierTypeFilter, ExoticTier)

	if len(reverseLightSortedItems) <= 1 {
		// TODO: If there are no other items from the specified character, then we need to figure out
		// an item to be transferred from the vault
		fmt.Println("No other items on the specified character, not currently setup to transfer new choices from the vault...")
		return
	}

	// Lowest light to highest
	sort.Sort(LightSort(reverseLightSortedItems))

	// Now that items are sorted in reverse light order, we want to equip the first item in the slice,
	// the highest light item will be the last item in the slice.
	itemToEquip := reverseLightSortedItems[0]
	character := item.Character
	equipItem(itemToEquip, character, membershipType, client)
}

func moveLoadoutToCharacter(loadout Loadout, destinationID string, characters CharacterList, membershipType int, client *Client) error {

	transferItem(loadout.toSlice(), characters, characters.findCharacterFromID(destinationID), membershipType, -1, client)

	return nil
}

// groupAndSortGear will return a map of ItemLists. The key of the map will be the bucket type
// of all of the items in the list. Each of the lists of items will be sorted by Light value.
func groupAndSortGear(inventory ItemList) map[EquipmentBucket]ItemList {

	result := make(map[EquipmentBucket]ItemList)

	result[Kinetic] = sortGearBucket(bucketHashLookup[Kinetic], inventory)
	result[Energy] = sortGearBucket(bucketHashLookup[Energy], inventory)
	result[Power] = sortGearBucket(bucketHashLookup[Power], inventory)
	result[Ghost] = sortGearBucket(bucketHashLookup[Ghost], inventory)

	result[Helmet] = sortGearBucket(bucketHashLookup[Helmet], inventory)
	result[Gauntlets] = sortGearBucket(bucketHashLookup[Gauntlets], inventory)
	result[Chest] = sortGearBucket(bucketHashLookup[Chest], inventory)
	result[Legs] = sortGearBucket(bucketHashLookup[Legs], inventory)
	result[ClassArmor] = sortGearBucket(bucketHashLookup[ClassArmor], inventory)
	//result[Artifact] = sortGearBucket(bucketHashLookup[Artifact], inventory)

	return result
}

func sortGearBucket(bucketHash uint, inventory ItemList) ItemList {

	result := inventory.FilterItems(itemBucketHashFilter, bucketHash)
	sort.Sort(sort.Reverse(LightSort(result)))
	return result
}

func findBestItemForBucket(bucket EquipmentBucket, items []*Item, destinationID string) *Item {

	if len(items) <= 0 {
		return nil
	}

	candidate := items[0]
	for i := 1; i < len(items); i++ {
		next := items[i]
		if next.Power() < candidate.Power() {
			// Lower light value, keep the current candidate
			break
		}

		if (next.Character != nil && next.CharacterID == destinationID) &&
			(candidate.Character != nil && candidate.CharacterID != destinationID) {
			// This next item is the same light and on the destination character already, the current candidate is not
			candidate = next
		} else if (next.Character != nil && next.CharacterID == destinationID) &&
			(candidate.Character != nil && candidate.CharacterID == destinationID) {
			if next.TransferStatus == ItemIsEquipped && candidate.TransferStatus != ItemIsEquipped {
				// The next item is currnetly equipped on the destination character, the current candidate is not
				candidate = next
			}
		} else if (candidate.Character != nil && candidate.CharacterID != destinationID) &&
			(next.Character == nil) {
			// If the current candidate is on a character that is NOT the destination and the next candidate is in the vault,
			// prefer that since we will only need to do a single transfer request
			candidate = next
		}
	}

	return candidate
}
