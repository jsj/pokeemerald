-- Pokéctx mGBA adapter for Pokémon Emerald (USA).
-- Load this script from Tools > Scripting in mGBA.

local ADDRESS = {
	saveBlock1Ptr = 0x03005D8C,
	saveBlock2Ptr = 0x03005D90,
	party = 0x020244EC,
	partyCount = 0x020244E9,
	saveCounter = 0x03006200,
	mainInBattle = 0x030026F9,
	battleTypeFlags = 0x02022FEC,
	battlersCount = 0x0202406C,
	battleMons = 0x02024084,
	cheatMailbox = 0x0203CF64,
}

local SIZE = { saveBlock1 = 0x3D88, saveBlock2 = 0x0F2C, partyMon = 100, battleMon = 0x58 }
local server = nil
local clients = {}
local nextClientID = 1
local pendingCheat = nil

local CHEAT_MAGIC = 0x50584354

local function hex(data)
	return (data:gsub('.', function(char) return string.format('%02x', string.byte(char)) end))
end

local function crc32()
	local bytes = emu:checksum(C.CHECKSUM.CRC32)
	return hex(bytes)
end

local function jsonEscape(value)
	return value:gsub('\\', '\\\\'):gsub('"', '\\"'):gsub('\n', '\\n'):gsub('\r', '\\r')
end

local function snapshot()
	local saveBlock1 = emu:read32(ADDRESS.saveBlock1Ptr)
	local saveBlock2 = emu:read32(ADDRESS.saveBlock2Ptr)
	local partyCount = emu:read8(ADDRESS.partyCount)
	if saveBlock1 == 0 or saveBlock2 == 0 then error('save blocks are not initialized') end
	if partyCount > 6 then error('party count is invalid') end
	local observedAt = os.date('!%Y-%m-%dT%H:%M:%SZ')
	local inBattle = emu:read8(ADDRESS.mainInBattle) & 0x02 ~= 0
	local battlersCount = inBattle and emu:read8(ADDRESS.battlersCount) or 0
	local battleMons = inBattle and hex(emu:readRange(ADDRESS.battleMons, battlersCount * SIZE.battleMon)) or ''
	return string.format(
		'{"protocol_version":1,"game_code":"%s","rom_crc32":"%s","observed_at":"%s","save_block_1_hex":"%s","save_block_2_hex":"%s","party_hex":"%s","party_count":%d,"save_counter":%u,"in_battle":%s,"battle_type_flags":%u,"battlers_count":%d,"battle_mons_hex":"%s"}\n',
		jsonEscape(emu:getGameCode()), crc32(), observedAt,
		hex(emu:readRange(saveBlock1, SIZE.saveBlock1)),
		hex(emu:readRange(saveBlock2, SIZE.saveBlock2)),
		hex(emu:readRange(ADDRESS.party, partyCount * SIZE.partyMon)),
		partyCount, emu:read32(ADDRESS.saveCounter), tostring(inBattle),
		inBattle and emu:read32(ADDRESS.battleTypeFlags) or 0, battlersCount, battleMons)
end

local function stopClient(id)
	local client = clients[id]
	clients[id] = nil
	if client then client:close() end
end

local function onClientError(id, message)
	console:error('Pokéctx client error: ' .. tostring(message))
	stopClient(id)
end

local function onClientData(id)
	local client = clients[id]
	if not client then return end
	local request, readError = client:receive(1024)
	if not request then
		if readError ~= socket.ERRORS.AGAIN then onClientError(id, readError) end
		return
	end
	request = request:match('^%s*(.-)%s*$')
	if request == 'snapshot' then
		local ok, response = pcall(snapshot)
		if not ok then response = '{"error":"' .. jsonEscape(tostring(response)) .. '"}\n' end
		client:send(response)
		stopClient(id)
		return
	end
	local command, arg1, arg2, nonce, dataHex = request:match('^cheat%s+(%d+)%s+(%d+)%s+(%d+)%s+(%d+)%s+([%x%-]+)$')
	if not command then
		client:send('{"error":"unknown request"}\n')
		stopClient(id)
		return
	end
	if pendingCheat or emu:read8(ADDRESS.cheatMailbox + 15) == 1 then
		client:send('{"error":"another cheat command is pending"}\n')
		stopClient(id)
		return
	end
	command, arg1, arg2, nonce = tonumber(command), tonumber(arg1), tonumber(arg2), tonumber(nonce)
	emu:write32(ADDRESS.cheatMailbox, 0)
	emu:write32(ADDRESS.cheatMailbox + 4, nonce)
	emu:write32(ADDRESS.cheatMailbox + 8, arg1)
	emu:write16(ADDRESS.cheatMailbox + 12, arg2)
	emu:write8(ADDRESS.cheatMailbox + 14, command)
	emu:write8(ADDRESS.cheatMailbox + 15, 1)
	emu:write32(ADDRESS.cheatMailbox + 16, 0)
	for offset = 0, 10 do emu:write8(ADDRESS.cheatMailbox + 20 + offset, 0xFF) end
	if dataHex ~= '-' then
		if not dataHex:match('^%x+$') or #dataHex % 2 ~= 0 or #dataHex > 22 then
			client:send('{"error":"invalid cheat data"}\n')
			stopClient(id)
			return
		end
		for offset = 0, (#dataHex / 2) - 1 do
			emu:write8(ADDRESS.cheatMailbox + 20 + offset, tonumber(dataHex:sub(offset * 2 + 1, offset * 2 + 2), 16))
		end
	end
	emu:write32(ADDRESS.cheatMailbox, CHEAT_MAGIC)
	pendingCheat = { id = id, nonce = nonce }
end

local function pollCheat()
	if not pendingCheat then return end
	local status = emu:read8(ADDRESS.cheatMailbox + 15)
	if status == 1 then return end
	local client = clients[pendingCheat.id]
	if client then
		client:send(string.format(
			'{"protocol_version":1,"nonce":%u,"status":%u,"result":%u}\n',
			pendingCheat.nonce, status, emu:read32(ADDRESS.cheatMailbox + 16)))
	end
	emu:write32(ADDRESS.cheatMailbox, 0)
	stopClient(pendingCheat.id)
	pendingCheat = nil
end

local function onAccept()
	local client, acceptError = server:accept()
	if acceptError then console:error('Pokéctx accept error: ' .. tostring(acceptError)); return end
	local id = nextClientID
	nextClientID = nextClientID + 1
	clients[id] = client
	client:add('received', function() onClientData(id) end)
	client:add('error', function(message) onClientError(id, message) end)
end

server = socket.bind('127.0.0.1', 8765)
if not server then error('Pokéctx cannot bind 127.0.0.1:8765') end
local ok, listenError = server:listen()
if not ok then server:close(); error('Pokéctx cannot listen: ' .. tostring(listenError)) end
server:add('received', onAccept)
callbacks:add('frame', pollCheat)
console:log('Pokéctx adapter listening on 127.0.0.1:8765')
