#include "global.h"
#include "field_screen_effect.h"
#include "event_data.h"
#include "evolution_scene.h"
#include "item.h"
#include "money.h"
#include "overworld.h"
#include "pokedex.h"
#include "pokemon.h"
#include "pokemon_storage_system.h"
#include "pokectx.h"
#include "random.h"
#include "script_pokemon_util.h"
#include "constants/characters.h"
#include "constants/map_groups.h"
#include "constants/maps.h"
#include "constants/flags.h"
#include "constants/pokemon.h"
#include "constants/moves.h"
#include "constants/species.h"

#define POKECTX_MAX_MONEY 999999
#define POKECTX_SHINY_FLAG (1u << 31)

static const u8 sMapGroupCounts[MAP_GROUPS_COUNT] =
{
    57, 5, 5, 6, 7, 8, 9, 7, 7, 14, 8, 17, 10, 23, 13, 15, 15,
    2, 2, 2, 3, 1, 1, 1, 108, 61, 89, 2, 1, 13, 1, 1, 3, 1,
};

extern const struct Evolution gEvolutionTable[NUM_SPECIES][EVOS_PER_MON];

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

static bool8 ForgetPartyMove(u8 partySlot, u16 move)
{
    struct Pokemon *mon = &gPlayerParty[partySlot];
    u16 moves[MAX_MON_MOVES];
    u8 count = 0;
    u8 i;
    u8 found = MAX_MON_MOVES;

    for (i = 0; i < MAX_MON_MOVES; i++)
    {
        moves[i] = GetMonData(mon, MON_DATA_MOVE1 + i, NULL);
        if (moves[i] != MOVE_NONE)
        {
            count++;
            if (moves[i] == move)
                found = i;
        }
    }
    if (found == MAX_MON_MOVES || count <= 1)
        return FALSE;
    for (i = found; i + 1 < MAX_MON_MOVES; i++)
        SetMonMoveSlot(mon, moves[i + 1], i);
    SetMonMoveSlot(mon, MOVE_NONE, MAX_MON_MOVES - 1);
    return TRUE;
}

static u8 GivePokemon(u16 species, u8 level, bool8 shiny)
{
    struct Pokemon mon;
    u32 personality;
    u32 otId;
    u16 nationalDexNum;
    u8 result;

    if (shiny)
    {
        otId = (u32)gSaveBlock2Ptr->playerTrainerId[0]
             | ((u32)gSaveBlock2Ptr->playerTrainerId[1] << 8)
             | ((u32)gSaveBlock2Ptr->playerTrainerId[2] << 16)
             | ((u32)gSaveBlock2Ptr->playerTrainerId[3] << 24);
        personality = Random32();
        personality = (personality & 0xFFFF)
                    | ((u32)(HIHALF(otId) ^ LOHALF(otId) ^ LOHALF(personality)) << 16);
        CreateMon(&mon, species, level, USE_RANDOM_IVS, TRUE, personality, OT_ID_PLAYER_ID, 0);
    }
    else
    {
        CreateMon(&mon, species, level, USE_RANDOM_IVS, FALSE, 0, OT_ID_PLAYER_ID, 0);
    }
    result = GiveMonToPlayer(&mon);
    if (result != MON_CANT_GIVE)
    {
        nationalDexNum = SpeciesToNationalPokedexNum(species);
        GetSetPokedexFlag(nationalDexNum, FLAG_SET_SEEN);
        GetSetPokedexFlag(nationalDexNum, FLAG_SET_CAUGHT);
    }
    return result;
}

static bool8 IsValidTeleportDestination(u8 mapGroup, u8 mapNum, u8 x, u8 y)
{
    const struct MapHeader *header;
    const struct MapLayout *layout;
    u16 block;
    u8 i;

    if (mapGroup >= MAP_GROUPS_COUNT || mapNum >= sMapGroupCounts[mapGroup])
        return FALSE;
    header = Overworld_GetMapHeaderByGroupAndId(mapGroup, mapNum);
    layout = header->mapLayout;
    if (layout == NULL || x >= layout->width || y >= layout->height)
        return FALSE;
    block = layout->map[x + y * layout->width];
    if (UNPACK_COLLISION(block) != 0)
        return FALSE;
    for (i = 0; i < header->events->objectEventCount; i++)
    {
        if (header->events->objectEvents[i].x == x
         && header->events->objectEvents[i].y == y)
            return FALSE;
    }
    return TRUE;
}

static u16 GetForcedEvolutionTarget(struct Pokemon *mon, u16 requestedSpecies)
{
    u16 species = GetMonData(mon, MON_DATA_SPECIES, NULL);
    u16 target = SPECIES_NONE;
    u8 i;

    for (i = 0; i < EVOS_PER_MON; i++)
    {
        u16 candidate = gEvolutionTable[species][i].targetSpecies;
        if (candidate == SPECIES_NONE)
            continue;
        if (requestedSpecies != SPECIES_NONE)
        {
            if (candidate == requestedSpecies)
                return candidate;
            continue;
        }
        if (target != SPECIES_NONE && target != candidate)
            return SPECIES_NONE;
        target = candidate;
    }
    return target;
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
    {
        bool8 shiny = (gPokectxCheatMailbox.arg1 & POKECTX_SHINY_FLAG) != 0;
        u16 species = gPokectxCheatMailbox.arg1 & 0xFFFF;

        if (species == SPECIES_NONE
         || species >= NUM_SPECIES
         || gPokectxCheatMailbox.arg2 == 0
         || gPokectxCheatMailbox.arg2 > MAX_LEVEL)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_INVALID_ARGUMENT;
            return;
        }
        gPokectxCheatMailbox.result = GivePokemon(species, gPokectxCheatMailbox.arg2, shiny);
        if (gPokectxCheatMailbox.result == MON_CANT_GIVE)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_STORAGE_FULL;
            return;
        }
        break;
    }
    case POKECTX_CHEAT_LEARN_MOVE:
        if (gPokectxCheatMailbox.arg1 >= gPlayerPartyCount
         || gPokectxCheatMailbox.arg2 == MOVE_NONE
         || gPokectxCheatMailbox.arg2 >= MOVES_COUNT)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_INVALID_ARGUMENT;
            return;
        }
        gPokectxCheatMailbox.result = GiveMoveToMon(
            &gPlayerParty[gPokectxCheatMailbox.arg1],
            gPokectxCheatMailbox.arg2);
        if (gPokectxCheatMailbox.result == MON_ALREADY_KNOWS_MOVE)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_ALREADY_KNOWS_MOVE;
            return;
        }
        if (gPokectxCheatMailbox.result == MON_HAS_MAX_MOVES)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_MOVESET_FULL;
            return;
        }
        break;
    case POKECTX_CHEAT_FORGET_MOVE:
        if (gPokectxCheatMailbox.arg1 >= gPlayerPartyCount
         || gPokectxCheatMailbox.arg2 == MOVE_NONE
         || gPokectxCheatMailbox.arg2 >= MOVES_COUNT)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_INVALID_ARGUMENT;
            return;
        }
        if (!ForgetPartyMove(gPokectxCheatMailbox.arg1, gPokectxCheatMailbox.arg2))
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_LAST_MOVE;
            return;
        }
        gPokectxCheatMailbox.result = gPokectxCheatMailbox.arg2;
        break;
    case POKECTX_CHEAT_HEAL:
        HealPlayerParty();
        gPokectxCheatMailbox.result = gPlayerPartyCount;
        break;
    case POKECTX_CHEAT_TELEPORT:
    {
        u8 mapGroup = gPokectxCheatMailbox.arg1 >> 8;
        u8 mapNum = gPokectxCheatMailbox.arg1;
        u8 x = gPokectxCheatMailbox.arg2;
        u8 y = gPokectxCheatMailbox.arg2 >> 8;

        if (!IsValidTeleportDestination(mapGroup, mapNum, x, y))
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_INVALID_DESTINATION;
            return;
        }
        SetWarpDestination(mapGroup, mapNum, WARP_ID_NONE, x, y);
        DoWarp();
        gPokectxCheatMailbox.result = gPokectxCheatMailbox.arg1;
        break;
    }
    case POKECTX_CHEAT_HOLD:
    {
        u16 item = gPokectxCheatMailbox.arg2;
        if (gPokectxCheatMailbox.arg1 >= gPlayerPartyCount
         || item >= ITEMS_COUNT)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_INVALID_ARGUMENT;
            return;
        }
        SetMonData(&gPlayerParty[gPokectxCheatMailbox.arg1], MON_DATA_HELD_ITEM, &item);
        gPokectxCheatMailbox.result = item;
        break;
    }
    case POKECTX_CHEAT_FRIENDSHIP:
    {
        u8 friendship = gPokectxCheatMailbox.arg2;
        if (gPokectxCheatMailbox.arg1 >= gPlayerPartyCount
         || gPokectxCheatMailbox.arg2 > 255)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_INVALID_ARGUMENT;
            return;
        }
        SetMonData(&gPlayerParty[gPokectxCheatMailbox.arg1], MON_DATA_FRIENDSHIP, &friendship);
        gPokectxCheatMailbox.result = friendship;
        break;
    }
    case POKECTX_CHEAT_CLONE:
    {
        struct Pokemon clone;
        struct Pokemon original;
        if (gPokectxCheatMailbox.arg1 >= gPlayerPartyCount)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_INVALID_ARGUMENT;
            return;
        }
        CopyMon(&original, &gPlayerParty[gPokectxCheatMailbox.arg1], sizeof(original));
        CopyMon(&clone, &original, sizeof(clone));
        gPokectxCheatMailbox.result = GiveMonToPlayer(&clone);
        if (gPokectxCheatMailbox.result == MON_CANT_GIVE)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_STORAGE_FULL;
            return;
        }
        if (gPokectxCheatMailbox.result == MON_GIVEN_TO_PARTY)
            CopyMon(&gPlayerParty[gPlayerPartyCount - 1], &original, sizeof(original));
        else
            CopyMon(GetBoxedMonPtr(gSpecialVar_MonBoxId, gSpecialVar_MonBoxPos), &original.box, sizeof(original.box));
        break;
    }
    case POKECTX_CHEAT_EVOLVE:
    {
        u16 target;
        if (gPokectxCheatMailbox.arg1 >= gPlayerPartyCount
         || gPokectxCheatMailbox.arg2 >= NUM_SPECIES)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_INVALID_ARGUMENT;
            return;
        }
        target = GetForcedEvolutionTarget(
            &gPlayerParty[gPokectxCheatMailbox.arg1],
            gPokectxCheatMailbox.arg2);
        if (target == SPECIES_NONE)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_NO_EVOLUTION;
            return;
        }
        BeginEvolutionScene(&gPlayerParty[gPokectxCheatMailbox.arg1], target, FALSE, gPokectxCheatMailbox.arg1);
        gPokectxCheatMailbox.result = target;
        break;
    }
    case POKECTX_CHEAT_RENAME:
    {
        u8 nickname[POKEMON_NAME_LENGTH + 1];
        u8 i;
        if (gPokectxCheatMailbox.arg1 >= gPlayerPartyCount)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_INVALID_ARGUMENT;
            return;
        }
        for (i = 0; i <= POKEMON_NAME_LENGTH; i++)
            nickname[i] = gPokectxCheatMailbox.data[i];
        nickname[POKEMON_NAME_LENGTH] = EOS;
        SetMonData(&gPlayerParty[gPokectxCheatMailbox.arg1], MON_DATA_NICKNAME, nickname);
        gPokectxCheatMailbox.result = gPokectxCheatMailbox.arg1;
        break;
    }
    case POKECTX_CHEAT_BADGE:
        if (gPokectxCheatMailbox.arg1 == 0xFF)
        {
            u8 i;
            for (i = 0; i < NUM_BADGES; i++)
                FlagSet(FLAG_BADGE01_GET + i);
            gPokectxCheatMailbox.result = NUM_BADGES;
            break;
        }
        if (gPokectxCheatMailbox.arg1 >= NUM_BADGES)
        {
            gPokectxCheatMailbox.status = POKECTX_CHEAT_INVALID_ARGUMENT;
            return;
        }
        FlagSet(FLAG_BADGE01_GET + gPokectxCheatMailbox.arg1);
        gPokectxCheatMailbox.result = gPokectxCheatMailbox.arg1;
        break;
    default:
        gPokectxCheatMailbox.status = POKECTX_CHEAT_INVALID_COMMAND;
        return;
    }

    gPokectxCheatMailbox.status = POKECTX_CHEAT_SUCCEEDED;
}
