package catalog

import (
	"fmt"
	"sort"
	"strings"
)

type MapEntry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MapGroup int    `json:"map_group"`
	MapNum   int    `json:"map_num"`
}

type Catalog struct {
	Maps      map[uint16]string
	Species   map[uint16]string
	Moves     map[uint16]string
	Items     map[uint16]string
	Abilities map[uint16]string
}

func (c Catalog) Map(group, number int) (string, string) {
	id := uint16(group<<8 | number)
	name, ok := c.Maps[id]
	if !ok {
		return fmt.Sprintf("MAP_%d_%d", group, number), fmt.Sprintf("Unknown map (%d,%d)", group, number)
	}
	return name, humanize(name)
}

func (c Catalog) MapEntries(query string) []MapEntry {
	wanted := normalize(query)
	entries := make([]MapEntry, 0, len(c.Maps))
	for packedID, id := range c.Maps {
		mapName := humanize(id)
		if wanted != "" && !strings.Contains(normalize(id), wanted) && !strings.Contains(normalize(mapName), wanted) {
			continue
		}
		entries = append(entries, MapEntry{
			ID:       id,
			Name:     mapName,
			MapGroup: int(packedID >> 8),
			MapNum:   int(packedID & 0xFF),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].MapGroup != entries[j].MapGroup {
			return entries[i].MapGroup < entries[j].MapGroup
		}
		return entries[i].MapNum < entries[j].MapNum
	})
	return entries
}

func (c Catalog) FindMap(value string) (MapEntry, bool) {
	wanted := normalize(value)
	for _, entry := range c.MapEntries("") {
		if normalize(entry.ID) == wanted || normalize(entry.Name) == wanted || normalize(strings.TrimPrefix(entry.ID, "MAP_")) == wanted {
			return entry, true
		}
	}
	return MapEntry{}, false
}

func (c Catalog) SpeciesName(id uint16) string { return name(c.Species, id, "species") }
func (c Catalog) MoveName(id uint16) string    { return name(c.Moves, id, "move") }
func (c Catalog) ItemName(id uint16) string    { return name(c.Items, id, "item") }
func (c Catalog) AbilityName(id uint16) string { return name(c.Abilities, id, "ability") }

func (c Catalog) FindItem(value string) (uint16, bool) {
	return find(c.Items, value)
}

func (c Catalog) FindSpecies(value string) (uint16, bool) {
	return find(c.Species, value)
}

func (c Catalog) FindMove(value string) (uint16, bool) {
	return find(c.Moves, value)
}

func find(values map[uint16]string, value string) (uint16, bool) {
	wanted := normalize(value)
	for id, candidate := range values {
		if normalize(candidate) == wanted || normalize(humanize(candidate)) == wanted {
			return id, true
		}
	}
	return 0, false
}

func normalize(value string) string {
	value = strings.ToUpper(value)
	return strings.NewReplacer("_", "", "-", "", " ", "").Replace(value)
}

func name(values map[uint16]string, id uint16, kind string) string {
	if value, ok := values[id]; ok {
		return humanize(value)
	}
	return fmt.Sprintf("Unknown %s %d", kind, id)
}

func humanize(value string) string {
	for _, prefix := range []string{"MAP_", "SPECIES_", "MOVE_", "ITEM_", "ABILITY_"} {
		if len(value) > len(prefix) && value[:len(prefix)] == prefix {
			value = value[len(prefix):]
		}
	}
	result := make([]byte, 0, len(value))
	upper := true
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if ch == '_' {
			result = append(result, ' ')
			upper = true
			continue
		}
		if upper && ch >= 'a' && ch <= 'z' {
			ch -= 'a' - 'A'
		} else if !upper && ch >= 'A' && ch <= 'Z' {
			ch += 'a' - 'A'
		}
		result = append(result, ch)
		upper = false
	}
	return string(result)
}
