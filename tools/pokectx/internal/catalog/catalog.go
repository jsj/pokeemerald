package catalog

import (
	"fmt"
	"strings"
)

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

func (c Catalog) SpeciesName(id uint16) string { return name(c.Species, id, "species") }
func (c Catalog) MoveName(id uint16) string    { return name(c.Moves, id, "move") }
func (c Catalog) ItemName(id uint16) string    { return name(c.Items, id, "item") }
func (c Catalog) AbilityName(id uint16) string { return name(c.Abilities, id, "ability") }

func (c Catalog) FindItem(value string) (uint16, bool) {
	wanted := normalize(value)
	for id, item := range c.Items {
		if normalize(item) == wanted || normalize(humanize(item)) == wanted {
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
