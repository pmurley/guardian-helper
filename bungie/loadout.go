package bungie

import (
	"fmt"
	"sort"
)

// Loadout will hold all items for a unique set of weapons, armor, ghost, class item, and artifact
type Loadout map[EquipmentBucket]*Item

func (l Loadout) calculateLightLevel() float64 {

	light := 0.0

	light += float64(l[Primary].PrimaryStat.Value) * 0.12
	light += float64(l[Special].PrimaryStat.Value) * 0.12
	light += float64(l[Heavy].PrimaryStat.Value) * 0.12
	light += float64(l[Ghost].PrimaryStat.Value) * 0.08

	light += float64(l[Helmet].PrimaryStat.Value) * 0.10
	light += float64(l[Arms].PrimaryStat.Value) * 0.10
	light += float64(l[Chest].PrimaryStat.Value) * 0.10
	light += float64(l[Legs].PrimaryStat.Value) * 0.10
	light += float64(l[ClassArmor].PrimaryStat.Value) * 0.08
	light += float64(l[Artifact].PrimaryStat.Value) * 0.08

	return light
}

func (l Loadout) toSlice() []*Item {

	result := make([]*Item, 0, Artifact-Primary)
	for i := Primary; i <= Artifact; i++ {
		result = append(result, l[i])
	}

	return result
}

func findMaxLightLoadout(itemsResponse *ItemsEndpointResponse, destinationIndex int) Loadout {
	// TODO: For now lets just use legendaries, later it should actually be !exotics which will require a new filter
	destinationClassType := itemsResponse.Response.Data.Characters[destinationIndex].CharacterBase.ClassType
	filteredItems := itemsResponse.Response.Data.Items.
		FilterItems(itemTierTypeFilter, SuperiorTier).
		FilterItems(itemClassTypeFilter, destinationClassType)
	gearSortedByLight := groupAndSortGear(filteredItems)

	// Find the best loadout given just legendary weapons
	loadout := make(Loadout)
	for i := Primary; i <= Artifact; i++ {
		loadout[i] = findBestItemForBucket(i, gearSortedByLight[i], 0)
	}

	// Determine the best exotics to use for both weapons and armor
	exotics := itemsResponse.Response.Data.Items.
		FilterItems(itemTierTypeFilter, ExoticTier).
		FilterItems(itemClassTypeFilter, destinationClassType)
	exoticsSortedAndGrouped := groupAndSortGear(exotics)

	// Override inventory items with exotics as needed
	for _, bucket := range [3]EquipmentBucket{Ghost, ClassArmor, Artifact} {
		exoticCandidate := findBestItemForBucket(bucket, exoticsSortedAndGrouped[bucket], destinationIndex)
		if exoticCandidate != nil && exoticCandidate.PrimaryStat.Value > loadout[bucket].PrimaryStat.Value {
			fmt.Printf("Overriding %s...\n", bucket)
			loadout[bucket] = exoticCandidate
		}
	}

	var weaponExoticCandidate *Item
	var weaponBucket EquipmentBucket
	for _, bucket := range [3]EquipmentBucket{Primary, Special, Heavy} {
		exoticCandidate := findBestItemForBucket(bucket, exoticsSortedAndGrouped[bucket], destinationIndex)
		if exoticCandidate != nil && exoticCandidate.PrimaryStat.Value > loadout[bucket].PrimaryStat.Value {
			if weaponExoticCandidate == nil || exoticCandidate.PrimaryStat.Value > weaponExoticCandidate.PrimaryStat.Value {
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
	for _, bucket := range [4]EquipmentBucket{Helmet, Arms, Chest, Legs} {
		exoticCandidate := findBestItemForBucket(bucket, exoticsSortedAndGrouped[bucket], destinationIndex)
		if exoticCandidate != nil && exoticCandidate.PrimaryStat.Value > loadout[bucket].PrimaryStat.Value {
			if armorExoticCandidate == nil || exoticCandidate.PrimaryStat.Value > armorExoticCandidate.PrimaryStat.Value {
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

func equipLoadout(loadout Loadout, destinationIndex int, characters []*Character, membershipType uint, client *Client) error {

	// Move all items to the destination character
	err := moveLoadoutToCharacter(loadout, destinationIndex, characters, membershipType)
	if err != nil {
		fmt.Println("Error moving loadout to destination character: ", err.Error())
		return err
	}

	// Equip all items that were just transferred
	equipItems(loadout.toSlice(), destinationIndex, characters, membershipType, client)

	return nil
}

func moveLoadoutToCharacter(loadout Loadout, destinationIndex int, characters []*Character, membershipType uint) error {

	// TODO: This membership ID should be dynamic
	transferItem(loadout.toSlice(), characters, characters[destinationIndex], membershipType, -1, NewClient("access-token", "token"))

	return nil
}

// groupAndSortGear will return a map of ItemLists. The key of the map will be the bucket type
// of all of the items in the list. Each of the lists of items will be sorted by Light value.
func groupAndSortGear(inventory ItemList) map[EquipmentBucket]ItemList {

	result := make(map[EquipmentBucket]ItemList)

	result[Primary] = sortGearBucket(bucketHashLookup[Primary], inventory)
	result[Special] = sortGearBucket(bucketHashLookup[Special], inventory)
	result[Heavy] = sortGearBucket(bucketHashLookup[Heavy], inventory)
	result[Ghost] = sortGearBucket(bucketHashLookup[Ghost], inventory)

	result[Helmet] = sortGearBucket(bucketHashLookup[Helmet], inventory)
	result[Arms] = sortGearBucket(bucketHashLookup[Arms], inventory)
	result[Chest] = sortGearBucket(bucketHashLookup[Chest], inventory)
	result[Legs] = sortGearBucket(bucketHashLookup[Legs], inventory)
	result[ClassArmor] = sortGearBucket(bucketHashLookup[ClassArmor], inventory)
	result[Artifact] = sortGearBucket(bucketHashLookup[Artifact], inventory)

	return result
}

func sortGearBucket(bucketHash uint, inventory ItemList) ItemList {

	result := inventory.FilterItems(bucketTypeHashFilter, bucketHash)
	sort.Sort(sort.Reverse(LightSort(result)))
	return result
}

func findBestItemForBucket(bucket EquipmentBucket, items []*Item, destinationIndex int) *Item {

	if len(items) <= 0 {
		return nil
	}

	candidate := items[0]
	for i := 1; i < len(items); i++ {
		next := items[i]
		if next.PrimaryStat.Value < candidate.PrimaryStat.Value {
			// Lower light value, keep the current candidate
			break
		}

		if next.CharacterIndex == destinationIndex && candidate.CharacterIndex != destinationIndex {
			// This next item is the same light and on the destination character already, the current candidate is not
			candidate = next
		} else if next.CharacterIndex == destinationIndex && candidate.CharacterIndex == destinationIndex {
			if next.TransferStatus == ItemIsEquipped && candidate.TransferStatus != ItemIsEquipped {
				// The next item is currnetly equipped on the destination character, the current candidate is not
				candidate = next
			}
		} else if candidate.CharacterIndex != destinationIndex && candidate.CharacterIndex != -1 && next.CharacterIndex == -1 {
			// If the current candidate is on a candidate that is NOT the destination and the next candidate is in the vault,
			// prefer that since we will only need to do a single transfer request
			candidate = next
		}
	}

	return candidate
}
