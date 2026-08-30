-- Pokéctx mGBA adapter for Pokémon Emerald (USA).
-- Load this script from Tools > Scripting in mGBA.

local ADDRESS = {
	saveBlock1Ptr = 0x03005D8C,
	saveBlock2Ptr = 0x03005D90,
	party = 0x020244EC,
	partyCount = 0x020244E9,
	saveCounter = 0x03006200,
}

local SIZE = { saveBlock1 = 0x3D88, saveBlock2 = 0x0F2C, partyMon = 100 }
local server = nil
local clients = {}
local nextClientID = 1

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
	return string.format(
		'{"protocol_version":1,"game_code":"%s","rom_crc32":"%s","observed_at":"%s","save_block_1_hex":"%s","save_block_2_hex":"%s","party_hex":"%s","party_count":%d,"save_counter":%u}\n',
		jsonEscape(emu:getGameCode()), crc32(), observedAt,
		hex(emu:readRange(saveBlock1, SIZE.saveBlock1)),
		hex(emu:readRange(saveBlock2, SIZE.saveBlock2)),
		hex(emu:readRange(ADDRESS.party, partyCount * SIZE.partyMon)),
		partyCount, emu:read32(ADDRESS.saveCounter))
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
	if request ~= 'snapshot' then
		client:send('{"error":"unknown request"}\n')
		stopClient(id)
		return
	end
	local ok, response = pcall(snapshot)
	if not ok then response = '{"error":"' .. jsonEscape(tostring(response)) .. '"}\n' end
	client:send(response)
	stopClient(id)
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
console:log('Pokéctx adapter listening on 127.0.0.1:8765')
