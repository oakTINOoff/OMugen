-- OMugen: Training Tools (training pause menu > Training Tools).
--
-- Draws over a training match what omugen_training.go measures: the input
-- display (each player's directions and buttons, newest at the top, with how
-- many frames they were held), the frame data of each player's last move
-- (startup, active and recovery frames, and the frame advantage after a hit
-- or a block), and the dummy's recording state.

local tr = {}

local W, H = 1280, 720
local font, text, back
local glyphs = {}

-- The screenpack's glyphs (as in the command list): directions by numpad
-- notation, forward to the right, and buttons.
local dirGlyph = {[1] = '_DB', [2] = '_D', [3] = '_DF', [4] = '_B', [6] = '_F', [7] = '_UB', [8] = '_U', [9] = '_UF'}
local buttonGlyph = {a = '^A', b = '^B', c = '^C', x = '^X', y = '^Y', z = '^Z', s = '^S', d = '^D', w = '^W'}
local dirText = {[1] = 'DB', [2] = 'D', [3] = 'DF', [4] = 'B', [5] = 'N', [6] = 'F', [7] = 'UB', [8] = 'U', [9] = 'UF'}

local function init()
	if font ~= nil then
		return
	end
	font = fontNew('data/omugen/fonts/b.def', 20)
	text = textImgNew()
	textImgSetFont(text, font)
	textImgSetBank(text, 0)
	textImgSetLayerno(text, 2)
	back = rectNew()
	rectSetLayerno(back, 2)
end

-- glyph is a copy of a screenpack glyph, drawn in this file's 1280x720
-- space, and its height in pixels.
local function glyph(token)
	if token == nil then
		return nil
	end
	if glyphs[token] == nil then
		glyphs[token] = false
		local g = motif.glyphs ~= nil and motif.glyphs[token] or nil
		if g ~= nil and g.AnimData ~= nil and g.Size ~= nil and g.Size[2] > 0 then
			local a = animCopy(g.AnimData)
			if a ~= nil then
				animSetLocalcoord(a, W, H)
				animSetLayerno(a, 2)
				animSetWindow(a, 0, 0, W, H)
				animUpdate(a) -- its sprite
				glyphs[token] = {anim = a, h = g.Size[2]}
			end
		end
	end
	return glyphs[token] or nil
end

local function box(x1, y1, x2, y2, c, alpha)
	rectSetLocalcoord(back, W, H)
	rectSetColor(back, c[1], c[2], c[3])
	rectSetAlpha(back, alpha, 255 - alpha)
	rectSetWindow(back, x1, y1, x2, y2)
	rectUpdate(back)
	rectDraw(back)
end

-- label draws a text; align: -1 right, 0 centered, 1 left.
local function label(s, x, y, scale, c, align)
	textImgSetLocalcoord(text, W, H)
	textImgSetAlign(text, align or 1)
	textImgSetScale(text, scale, scale)
	textImgSetColor(text, c[1], c[2], c[3], 255)
	textImgSetText(text, s)
	textImgSetPos(text, x, y)
	textImgDraw(text)
end

local function drawGlyph(token, x, y, size)
	local g = glyph(token)
	if g == nil then
		return false
	end
	animSetScale(g.anim, size / g.h, size / g.h)
	animSetPos(g.anim, x, y)
	animDraw(g.anim)
	return true
end

-- One player's input display: a column of the last inputs, newest first.
local function inputs(hist, x)
	local rows = math.min(#hist, 15)
	if rows == 0 then
		return
	end
	local top, step = 178, 30
	box(x - 8, top - 4, x + 250, top + rows * step + 4, {0, 0, 0}, 140)
	for i = 1, rows do
		local e = hist[i]
		local base = top + i * step - 6
		local fade = i == 1 and {255, 255, 255} or {190, 190, 200}
		label(tostring(e.frames), x + 36, base - 2, 0.8, fade, -1)
		local gx = x + 46
		if e.dir ~= 5 then
			if not drawGlyph(dirGlyph[e.dir], gx, base, 26) then
				label(dirText[e.dir], gx, base - 2, 0.8, fade, 1)
			end
		end
		gx = gx + 32
		for ch in e.buttons:gmatch('.') do
			if not drawGlyph(buttonGlyph[ch], gx, base, 26) then
				label(ch:upper(), gx, base - 2, 0.8, fade, 1)
			end
			gx = gx + 28
		end
	end
end

local function signed(n)
	if n > 0 then
		return '+' .. n
	end
	return tostring(n)
end

-- One player's frame data, centered on x.
local function frames(f, x, pn)
	local s = string.format('P%d  Startup %s   Active %d   Recovery %d',
		pn, f.startup > 0 and tostring(f.startup) or '-', f.active, f.recovery)
	box(x - 250, 118, x + 250, f.contact ~= '' and 186 or 154, {0, 0, 0}, 150)
	label(s, x, 146, 0.85, {255, 255, 255}, 0)
	if f.contact ~= '' then
		local what = f.contact == 'hit' and 'On hit' or 'On block'
		local adv, c = signed(f.adv), {255, 255, 255}
		if f.knockdown then
			adv, c = 'knockdown', {255, 210, 90}
		elseif f.adv > 0 then
			c = {90, 230, 120}
		elseif f.adv < 0 then
			c = {255, 95, 95}
		end
		label(what .. '  ' .. adv, x, 178, 0.85, c, 0)
	end
end

function tr.draw()
	if omugenTrainingInfo == nil or main.pauseMenuActive then
		return
	end
	local t = omugenTrainingInfo()
	init()
	if t.recording then
		box(W / 2 - 120, 92, W / 2 + 120, 128, {0, 0, 0}, 170)
		box(W / 2 - 104, 104, W / 2 - 92, 116, {235, 40, 50}, 255)
		label(string.format('REC  %.1f  (pause to stop)', t.recordLeft), W / 2 - 84, 119, 0.75, {255, 255, 255}, 1)
	elseif t.playback then
		box(W / 2 - 90, 92, W / 2 + 90, 128, {0, 0, 0}, 150)
		label(string.format('PLAYBACK  %.1fs', t.recorded), W / 2, 119, 0.75, {120, 220, 255}, 0)
	end
	if t.inputs >= 1 then
		inputs(t[1].history, 24)
	end
	if t.inputs >= 2 then
		inputs(t[2].history, W - 274)
	end
	if t.framedata then
		if t[1].frames ~= nil then
			frames(t[1].frames, 330, 1)
		end
		if t[2].frames ~= nil then
			frames(t[2].frames, W - 330, 2)
		end
	end
end

hook.add('loop#training', 'omugenTrainingTools', tr.draw)

return tr
