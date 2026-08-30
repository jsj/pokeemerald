#ifndef GUARD_POKECTX_H
#define GUARD_POKECTX_H

#define POKECTX_CHEAT_MAGIC 0x50584354 // "PXCT"

enum PokectxCheatCommand
{
    POKECTX_CHEAT_NONE,
    POKECTX_CHEAT_SET_MONEY,
    POKECTX_CHEAT_SET_LEVEL,
    POKECTX_CHEAT_GIVE_ITEM,
};

enum PokectxCheatStatus
{
    POKECTX_CHEAT_PENDING = 1,
    POKECTX_CHEAT_SUCCEEDED,
    POKECTX_CHEAT_INVALID_COMMAND,
    POKECTX_CHEAT_INVALID_ARGUMENT,
    POKECTX_CHEAT_BAG_FULL,
};

struct PokectxCheatMailbox
{
    u32 magic;
    u32 nonce;
    u32 arg1;
    u16 arg2;
    u8 command;
    u8 status;
    u32 result;
};

extern volatile struct PokectxCheatMailbox gPokectxCheatMailbox;

void Pokectx_ProcessCheatMailbox(void);

#endif // GUARD_POKECTX_H
