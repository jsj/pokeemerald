#include "global.h"
#include "item.h"
#include "money.h"
#include "pokemon.h"
#include "pokectx.h"
#include "script_pokemon_util.h"
#include "constants/pokemon.h"
#include "constants/species.h"

#define POKECTX_MAX_MONEY 999999

EWRAM_DATA volatile struct PokectxCheatMailbox gPokectxCheatMailbox = {0};

static void SetPartyLevel(u8 slot, u8 level)
{
    struct Pokemon *mon = &gPlayerParty[slot];
    u16 species = GetMonData(mon, MON_DATA_SPECIES, NULL);
    u32 experience = gExperienceTables[gSpeciesInfo[species].growthRate][level];

    SetMonData(mon, MON_DATA_EXP, &experience);
    CalculateMonStats(mon);
    SetMonData(mon, MON_DATA_HP, &mon->maxHP);
}

void Pokectx_ProcessCheatMailbox(void)
{
    if (gPokectxCheatMailbox.magic != POKECTX_CHEAT_MAGIC
     || gPokectxCheatMailbox.status != POKECTX_CHEAT_PENDING)
        return;

    switch (gPokectxCheatMailbox.command)
    {
    case POKECTX_CHEAT_SET_MONEY:
        if (gPokectxCheatMailbox.arg1 > POKECTX_MAX_MONEY)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_INVALID_ARGUMENT;
            return;
        }
        SetMoney(&gSaveBlock1Ptr->money, gPokectxCheatMailbox.arg1);
        gPokectxCheatMailbox.result = gPokectxCheatMailbox.arg1;
        break;
    case POKECTX_CHEAT_SET_LEVEL:
        if (gPokectxCheatMailbox.arg1 >= gPlayerPartyCount
         || gPokectxCheatMailbox.arg2 == 0
         || gPokectxCheatMailbox.arg2 > MAX_LEVEL)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_INVALID_ARGUMENT;
            return;
        }
        SetPartyLevel(gPokectxCheatMailbox.arg1, gPokectxCheatMailbox.arg2);
        gPokectxCheatMailbox.result = gPokectxCheatMailbox.arg2;
        break;
    case POKECTX_CHEAT_GIVE_ITEM:
        if (gPokectxCheatMailbox.arg1 == ITEM_NONE
         || gPokectxCheatMailbox.arg1 >= ITEMS_COUNT
         || gPokectxCheatMailbox.arg2 == 0)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_INVALID_ARGUMENT;
            return;
        }
        if (!AddBagItem(gPokectxCheatMailbox.arg1, gPokectxCheatMailbox.arg2))
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_BAG_FULL;
            return;
        }
        gPokectxCheatMailbox.result = gPokectxCheatMailbox.arg2;
        break;
    case POKECTX_CHEAT_GIVE_POKEMON:
        if (gPokectxCheatMailbox.arg1 == SPECIES_NONE
         || gPokectxCheatMailbox.arg1 >= NUM_SPECIES
         || gPokectxCheatMailbox.arg2 == 0
         || gPokectxCheatMailbox.arg2 > MAX_LEVEL)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_INVALID_ARGUMENT;
            return;
        }
        gPokectxCheatMailbox.result = ScriptGiveMon(
            gPokectxCheatMailbox.arg1,
            gPokectxCheatMailbox.arg2,
            ITEM_NONE,
            0,
            0,
            0);
        if (gPokectxCheatMailbox.result == MON_CANT_GIVE)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_STORAGE_FULL;
            return;
        }
        break;
    default:
        gPokectxCheatMailbox.status = POKECTX_CHEAT_INVALID_COMMAND;
        return;
    }

    gPokectxCheatMailbox.status = POKECTX_CHEAT_SUCCEEDED;
}
